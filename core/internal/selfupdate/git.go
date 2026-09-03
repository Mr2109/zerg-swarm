package selfupdate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// gitTimeout —— 单条 git 网络命令的上限（fetch/ls-remote）。被动检查不能把启动拖死。
const gitTimeout = 60 * time.Second

// nonInteractiveGitEnv —— 网络类 git 子命令的环境：禁止任何交互式提问/凭据弹窗。
// 依据：被动更新检查在 GUI 宿主/后台进程里跑，一旦 git 弹 "Username for 'https://…'"
// 会**永久挂死**（Hermes 的 network=True 分支同理）。GIT_TERMINAL_PROMPT=0 + 空 askpass。
func nonInteractiveGitEnv() []string {
	return append([]string{
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ASKPASS=",
		"SSH_ASKPASS=",
		"GCM_INTERACTIVE=never",
	}, "PATH="+pathEnv(), "HOME="+homeEnv())
}

func pathEnv() string { return envOr("PATH", "/usr/bin:/bin:/usr/sbin:/sbin") }
func homeEnv() string { return envOr("HOME", "/tmp") }

func envOr(k, def string) string {
	if v, ok := os.LookupEnv(k); ok && v != "" {
		return v
	}
	return def
}

// Git —— 一个本地检出的只读/增量句柄。
type Git struct{ Dir string }

func (g Git) String() string { return g.Dir }

// run 跑一条 git 命令；返回去空白 stdout。网络命令带超时与非交互环境。
func (g Git) run(network bool, args ...string) (string, error) {
	ctx := context.Background()
	var cancel context.CancelFunc
	if network {
		ctx, cancel = context.WithTimeout(ctx, gitTimeout)
		defer cancel()
	}
	full := append([]string{"-C", g.Dir}, args...)
	cmd := exec.CommandContext(ctx, "git", full...)
	if network {
		cmd.Env = nonInteractiveGitEnv()
		cmd.Stdin = nil
	}
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		return strings.TrimSpace(out.String()), fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return strings.TrimSpace(out.String()), nil
}

// HeadSHA —— 当前 HEAD 的完整 sha。
func (g Git) HeadSHA() (string, error) { return g.run(false, "rev-parse", "HEAD") }

// IsShallow —— 是否浅检出（installer 用 `git clone --depth 1` 建的）。
func (g Git) IsShallow() bool {
	out, err := g.run(false, "rev-parse", "--is-shallow-repository")
	return err == nil && out == "true"
}

// IsDirty —— 工作树是否有未提交改动（判定"开发态"用）。
func (g Git) IsDirty() bool {
	out, err := g.run(false, "status", "--porcelain")
	return err == nil && strings.TrimSpace(out) != ""
}

// IsAncestor —— a 是否为 b 的祖先（同一历史的包含关系判定）。
// 浅检出跨边界时 git 会报错 ⇒ 返回 (false,false)，由调用方按"判不出"处理（不猜）。
func (g Git) IsAncestor(a, b string) (isAncestor bool, ok bool) {
	if a == "" || b == "" {
		return false, false
	}
	_, err := g.run(false, "merge-base", "--is-ancestor", a, b)
	if err == nil {
		return true, true
	}
	// 退出码 1 = 确证"不是祖先"；其它（128 等）= 历史不足，判不出。
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() == 1 {
		return false, true
	}
	return false, false
}

// CountAhead —— from..to 的提交数（判不出返回 -1）。
func (g Git) CountAhead(from, to string) int {
	out, err := g.run(false, "rev-list", "--count", from+".."+to)
	if err != nil {
		return -1
	}
	n := -1
	fmt.Sscanf(out, "%d", &n)
	return n
}

// FetchTarget —— **scoped fetch**（只拉一个 ref），保持浅检出边界，返回 FETCH_HEAD 的 sha。
//
// 为什么必须 scoped：不加范围的 `git fetch origin` 会拉全部远端分支（实测 ~1400 个 head，
// 3.0s vs 0.55s），且会 **unshallow** 浅检出、让 `rev-list HEAD..origin/main` 报出
// 假的"落后 12492 笔"（Hermes banner.py 注释里踩过这个坑）。`--depth 1` 保住浅边界。
//
// 浅检出的 `clone --depth 1` 不建 `origin/main` 跟踪引用 ⇒ 优先 FETCH_HEAD。
// OriginRevID —— 读 ref 的提交信息里的 `GitOrigin-RevId` trailer（镜像仓用它回指私有提交）。
//
// 2026-09-13（真机验收）：公开仓是**过滤镜像**，逐提交都被重写 ⇒ 私有与公开两条历史在 sha 图上
// 互不为祖先，直接 rev-list 会报「落后 = 整部公开史」（实测 393 笔），而真相应是「已是最新」。
// 口径：镜像 tip 带 `GitOrigin-RevId: <私有 sha>` ⇒ 拿它映射回私有图再比祖先。读不到就返回空串。
func (g Git) OriginRevID(ref string) string {
	cmd := exec.Command("git", "-C", g.Dir, "log", "-1",
		"--format=%(trailers:key=GitOrigin-RevId,valueonly)", ref)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// DefaultPublicRepoURL —— 更新取源的**内置默认**：公开镜像仓（匿名可拉，无需凭据）。
//
// 2026-09-13 真机只读验收发现的问题：私有权威仓**没有 `origin`**（它一直是独立仓），
// 而取源原先只认 "origin" ⇒ `zerg update` 在真机上一次都跑不通。沙箱里的玩具仓恰好配了
// origin，所以套件全绿也验不出来 —— 平台/环境常量类的缺口，必须拿真仓跑一次才现形。
const DefaultPublicRepoURL = "https://github.com/Mr2109/zerg-swarm.git"

// ResolveUpdateRemote —— 取源解析顺序：① 环境变量 ZERG_UPDATE_REMOTE
// ② 已配置的 `origin`（尊重本机既有配置）③ 内置公开仓 URL（设计稿口径：匿名拉公开仓）。
func ResolveUpdateRemote(repoDir string) string {
	if v := strings.TrimSpace(os.Getenv("ZERG_UPDATE_REMOTE")); v != "" {
		return v
	}
	cmd := exec.Command("git", "-C", repoDir, "remote", "get-url", "origin")
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := cmd.Output(); err == nil && strings.TrimSpace(string(out)) != "" {
		return "origin"
	}
	return DefaultPublicRepoURL
}

func (g Git) FetchTarget(remote, ref string) (string, error) {
	if strings.TrimSpace(remote) == "" {
		remote = "origin"
	}
	if strings.TrimSpace(ref) == "" {
		ref = "main"
	}
	args := []string{"fetch", "--quiet"}
	if g.IsShallow() {
		args = append(args, "--depth", "1")
	}
	args = append(args, remote, ref)
	if _, err := g.run(true, args...); err != nil {
		return "", err
	}
	if sha, err := g.run(false, "rev-parse", "FETCH_HEAD"); err == nil && sha != "" {
		return sha, nil
	}
	// 退路：远端跟踪引用（非浅检出/已配置 remote 的情况）。
	if sha, err := g.run(false, "rev-parse", remote+"/"+ref); err == nil && sha != "" {
		return sha, nil
	}
	return "", fmt.Errorf("fetch 成功但取不到目标 sha（remote=%s ref=%s）", remote, ref)
}

// LSRemote —— 只读探远端 ref（离线/无凭据时返回错误，供"不确定不缓存"使用）。
func (g Git) LSRemote(url, ref string) (string, error) {
	out, err := g.run(true, "ls-remote", url, ref)
	if err != nil {
		return "", err
	}
	f := strings.Fields(out)
	if len(f) == 0 {
		return "", fmt.Errorf("ls-remote 无输出（url=%s ref=%s）", url, ref)
	}
	return f[0], nil
}

// WorktreeAddDetached —— 在临时区落一个 detached 检出（**绝不碰主工作树**，对照 G4）。
// 目标 commit 若已在本地对象库（刚 fetch 过）即可；失败则返回错误（不静默回退到 checkout）。
func (g Git) WorktreeAddDetached(dest, rev string) error {
	_, err := g.run(false, "worktree", "add", "--detach", "--force", dest, rev)
	return err
}

// WorktreeRemove —— 清理临时检出（尽力而为）。
func (g Git) WorktreeRemove(dest string) {
	_, _ = g.run(false, "worktree", "remove", "--force", dest)
	_, _ = g.run(false, "worktree", "prune")
}
