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

func TestResolveUpdateRemote_FallsBackToLocalAuthoritativeRepo(t *testing.T) {
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q")
	t.Setenv("ZERG_UPDATE_REMOTE", "")
	if got := ResolveUpdateRemote(dir); got != dir {
		t.Fatalf("无 origin 时应回落到**本机权威仓**（检出自身），✗ 不是公开源：got %q want %q", got, dir)
	}
}

func TestResolveUpdateRemote_PublicURLOnlyWhenNotGitCheckout(t *testing.T) {
	dir := t.TempDir() // 非 git 检出：公开 URL 只剩这一格兜底
	t.Setenv("ZERG_UPDATE_REMOTE", "")
	if got := ResolveUpdateRemote(dir); got != DefaultPublicRepoURL {
		t.Fatalf("仅非 git 检出才回落内置公开仓 URL：got %q want %q", got, DefaultPublicRepoURL)
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

// ── GAP-20261013-05：已装件自报 commit 的判定（检出持平之后的第二比）─────────────

// fakeBin —— 造一枚假的已装制品：`<name> --version` 回一行 <line>；置为可执行。
func fakeBin(t *testing.T, dir, name, line string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\necho \""+line+"\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

// 态 a：检出已持平，但已装件仍在旧版 ⇒ 判需更新（否则就是真病：节点机永不换装）。
func TestInstalledUpdateVerdict_CheckoutTiedInstalledOld_NeedUpdate(t *testing.T) {
	target := "ea8871b3c0ffee00000000000000000000000000" // 目标（镜像 tip，40 位）
	prefix := t.TempDir()
	// 已装件自报旧 commit（8 位短 sha —— 实测 mini-node-2 形态：在跑 2.5.13 / e830584b）
	fakeBin(t, prefix, "zerg-core", "zerg-core 2.5.13 e830584b 2026-09-24T00:00:00Z")
	need, why := installedUpdateVerdict(prefix, []string{"core", "agentd"}, target)
	if !need {
		t.Fatalf("检出持平但已装件旧 ⇒ 必须判需更新（真病：永不换装）；why=%q", why)
	}
	if !strings.Contains(why, "已装件落后") {
		t.Fatalf("判需更新时应点明「已装件落后」：%q", why)
	}
}

// 态 b：两端都新（检出持平 + 已装件自报 commit == 目标）⇒ 仍 rc=2「无需更新」。
func TestInstalledUpdateVerdict_BothNew_StillUpToDate(t *testing.T) {
	target := "1a6b02fc11223344556677889900112233445566"
	prefix := t.TempDir()
	fakeBin(t, prefix, "zerg-core", "zerg-core 2.5.15 1a6b02fc 2026-10-12T14:03:29Z")
	fakeBin(t, prefix, "zerg-agentd", "zerg-agentd 2.5.15 1a6b02fc 2026-10-12T14:03:29Z")
	need, why := installedUpdateVerdict(prefix, []string{"core", "agentd"}, target)
	if need {
		t.Fatalf("两端都新 ⇒ 不得判需更新（应 rc=2）；why=%q", why)
	}
}

// ui 的 key=value 形态（commit=<sha>+dirty）也要能解析并与目标相等。
func TestInstalledUpdateVerdict_UIKeyValueFormat(t *testing.T) {
	target := "1a6b02fc11223344556677889900112233445566"
	prefix := t.TempDir()
	fakeBin(t, prefix, "zerg-ui", "[zerg-ui] version=2.5.15 commit=1a6b02fc+dirty build=2026-10-12T14:04:28Z")
	if need, why := installedUpdateVerdict(prefix, []string{"ui"}, target); need {
		t.Fatalf("ui 的 key=value 形态应能解析并与目标相等 ⇒ 不判需更新；why=%q", why)
	}
}

// 一件都读不到（未安装）⇒ 判不了 ⇒ 按需更新（不能证明已装件达标 ⇒ 不宣布「无需更新」）。
func TestInstalledUpdateVerdict_Unreadable_NeedUpdate(t *testing.T) {
	if need, why := installedUpdateVerdict(t.TempDir(), []string{"core"}, "1a6b02fc1122"); !need {
		t.Fatalf("读不到任何已装件自报 commit ⇒ 不得宣布无需更新；why=%q", why)
	}
}

// 单件读不到（茧壁不带 commit 格）只是跳过该件、不单独触发更新；能报的件达标即真无需更新。
func TestInstalledUpdateVerdict_NonReportingComponentSkipped(t *testing.T) {
	target := "1a6b02fc11223344556677889900112233445566"
	prefix := t.TempDir()
	fakeBin(t, prefix, "zerg-wall", "zerg-wall 2.5.15")
	fakeBin(t, prefix, "zerg-core", "zerg-core 2.5.15 1a6b02fc 2026-10-12T14:03:29Z")
	if need, why := installedUpdateVerdict(prefix, []string{"core", "wall"}, target); need {
		t.Fatalf("不自报 commit 的件应被跳过而非触发更新；why=%q", why)
	}
}

// ── GAP-20261013-12：降级守卫 Go 侧（run() 丢退码 ⇒ IsAncestor 判不出非祖先 ⇒ StatusDiverged 死代码）──

// divergedRepos —— 造一对「真分叉」的 本地检出 / 裸远端：共同祖先 c1；本地在 c1 上加一笔 c2；
// 远端侧另起一棵树在 c1 上加一笔 c3 并推回。⇒ 各有独占提交、互不为祖先
// （＝公开↔私有混态在 sha 图上的形态；真仓实测：main tip 7542de85 与本仓 HEAD 两向 --is-ancestor 皆 rc=1）。
func divergedRepos(t *testing.T) (local, remote string) {
	t.Helper()
	local, remote = seedAndClone(t, 1) // 共同祖先 c1；本地 HEAD = 远端 tip = c1
	commitFile(t, local, "local.txt", "L", "local-only")
	other := filepath.Join(t.TempDir(), "other")
	gitRun(t, filepath.Dir(other), "clone", "-q", "-b", "main", remote, other)
	commitFile(t, other, "remote.txt", "R", "remote-only")
	gitRun(t, other, "push", "-q", "origin", "HEAD:refs/heads/main")
	return local, remote
}

// 态①「确证非祖先 ⇒ 可判」：真分叉下两支互不为祖先 ⇒ IsAncestor 两向都必须回 (false,true)
// （确证），✗ 不是 (false,false)（判不出）—— 改前 run() 丢退码 ⇒ errors.As 永不中 ⇒ 恒 (false,false)。
func TestIsAncestor_ConclusiveNonAncestor(t *testing.T) {
	local, remote := divergedRepos(t)
	g := Git{Dir: local}
	remoteSHA, err := g.FetchTarget(remote, "main")
	if err != nil {
		t.Fatalf("fetch 失败：%v", err)
	}
	headSHA, err := g.HeadSHA()
	if err != nil {
		t.Fatalf("读 HEAD 失败：%v", err)
	}
	if is, ok := g.IsAncestor(remoteSHA, headSHA); !ok || is {
		t.Fatalf("真分叉：is-ancestor(远端, 本地) 必须 (false,true)（确证非祖先）；got (%v,%v)", is, ok)
	}
	if is, ok := g.IsAncestor(headSHA, remoteSHA); !ok || is {
		t.Fatalf("真分叉：is-ancestor(本地, 远端) 必须 (false,true)；got (%v,%v)", is, ok)
	}
}

// 态②「祖先 ⇒ 仍正常」：本地是远端祖先（少一笔）⇒ IsAncestor (true,true)，Compare 仍判 StatusBehind。
func TestIsAncestor_TrueAncestor(t *testing.T) {
	local, remote := seedAndClone(t, 2)
	g := Git{Dir: local}
	gitRun(t, local, "reset", "-q", "--hard", "HEAD~1") // 本地退回一笔 ⇒ 本地成为远端祖先
	headSHA, _ := g.HeadSHA()
	remoteSHA, err := g.FetchTarget(remote, "main")
	if err != nil {
		t.Fatalf("fetch 失败：%v", err)
	}
	if is, ok := g.IsAncestor(headSHA, remoteSHA); !ok || !is {
		t.Fatalf("本地是远端祖先 ⇒ 必须 (true,true)；got (%v,%v)", is, ok)
	}
	if r := Compare(headSHA, remoteSHA, g.IsAncestor, 0); r.Status != StatusBehind || !r.Updatable() {
		t.Fatalf("本地是远端祖先 ⇒ 必须仍判 StatusBehind（正常升级）；got %+v", r)
	}
}

// 态③「真分叉 ⇒ 非 behind」：真分叉下 Compare 必须判 StatusDiverged —— ✗ 不是 behind
// （改前落 fallback 当 behind ⇒ Run() 的 switch 无 case 落空 ⇒ 误降级放行）。
func TestCompare_RealDivergeIsNotBehind(t *testing.T) {
	local, remote := divergedRepos(t)
	g := Git{Dir: local}
	remoteSHA, err := g.FetchTarget(remote, "main")
	if err != nil {
		t.Fatalf("fetch 失败：%v", err)
	}
	headSHA, _ := g.HeadSHA()
	r := Compare(headSHA, remoteSHA, g.IsAncestor, 0)
	if r.Status != StatusDiverged {
		t.Fatalf("真分叉（互不为祖先）⇒ 必须 StatusDiverged，✗ 不是 %s；got %+v", StatusBehind, r)
	}
}

// 端到端：真分叉下 Run() 必须**拦下**（rc=3 且不构建换装），并说清「两边历史无共同祖先」。
func TestRun_DivergedRefused(t *testing.T) {
	local, remote := divergedRepos(t)
	var out, errb strings.Builder
	o := Options{
		RepoDir: local, Remote: remote, Ref: "main",
		Prefix:    t.TempDir(),
		Kernel:    filepath.Join(t.TempDir(), "no-such-kernel.sh"),
		StateDir:  t.TempDir(),
		Receipts:  t.TempDir(),
		FleetYAML: filepath.Join(t.TempDir(), "fleet.yaml"),
		Role:      RoleController,
		Stdout:    &out, Stderr: &errb,
	}
	if rc := Run(o); rc != ExitRefused {
		t.Fatalf("真分叉必须拦下（rc=%d，且不构建换装）；got rc=%d\nstdout=%s\nstderr=%s",
			ExitRefused, rc, out.String(), errb.String())
	}
	if !strings.Contains(errb.String(), "无共同祖先") {
		t.Fatalf("拦下时应说清「两边历史无共同祖先」；stderr=%q", errb.String())
	}
	if strings.Contains(out.String(), "目标：") {
		t.Fatalf("拦下后不得再打「🏷 目标」进入构建；stdout=%q", out.String())
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
