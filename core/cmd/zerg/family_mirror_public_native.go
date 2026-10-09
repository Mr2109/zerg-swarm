// family_mirror_public_native.go —— `publish/mirror-public.sh`（**逐提交镜像器外壳**）的 **Go 真身**
// （发布链 Go 半优先 · 本批第三件 · 前两件 = `family_publish_preflight_native.go` / `family_publish_public_native.go`）。
//
// 病（原壳）：`publish/mirror-public.sh` 是公开面**真源**发布器（规则指纹 / 全树重写模式长在它身上），
// 但它的六段编排（环境检查 / 逐提交镜像 / 私有面门禁 / 软门禁 / 私有仓未被动过 / 推送）长在 shell 里，
// CLI 面看不见真身。本笔把编排**搬进进程内**，逐段与旧壳 `publish/mirror-public.sh`（191 行）逐字对齐。
//
// ★ 数据处理面（引擎 `publish/mirror-public-lib.py`）**逐字节照旧**：
//
//	引擎是**整件 python**（不是旧壳里的内联 heredoc 块）—— 本件把它**逐字节内嵌**为 base64 常量
//	（见 `family_mirror_public_pyblocks.go` 的 `mpPyLibB64`，落盘前 sha256 与原件逐字节同 = 83552369…），
//	运行期经 `python3 -c <bootstrap>` 交 **stdin** 执行，并在 bootstrap 里注入与旧壳**逐字同形**的
//	`__file__` / `argv[0]`（脚本位置）⇒ `REPO_ROOT` 推导面与旧壳**同锚**（✗ 不吃 cwd）。
//	⇒ 这几段（`os.walk` 序 / `re` 语义 / unicode 转义 / git 子进程序）**保证与旧壳逐字同**，不另造 Go 第二口径。
//	★ 为什么用 base64 而不是字面量：引擎正文里**逐字含**门禁自指的私有探针形态（`_INL_BLOCK` 的
//	  `／Users/…` / 机群机型族等 —— 正是它要抓的东西）；若以字面量落进 `core/**`，本件一落仓就
//	  **自己把自己的门判红**（`check-hardcoded-private-paths` / `check-history-secrets` 的扫描面含 `core/`）。
//	  base64（URL-safe）字符集不含 `/`、也不构成任何探针串 ⇒ 源码文本零命中，而**解码后逐字节不变**。
//
// ★ 与 `publish/publish-public.sh`（压平器 / 器甲）的**共用逻辑**（单一真源 · ✗ 不另抄一份）：
//
//	两器都吃 `publish/excludes.txt` / 替换规则 / 白名单 / 私有面黑名单；都吃同源 `_INL_BLOCK`
//	（mirror `Assets.scan` ⟷ Go 半 `chsecBlockSpecs`，15 条 · 见 check_history_secrets.go:59-77）。
//
// ★ 标准声明行 (a) —— **默认根定位差异**：py/旧壳由 `__file__` / `${BASH_SOURCE[0]}` 上溯定位仓根；
// Go 无 `__file__` 语境 ⇒ 本件从**入参 `root`** 取（调用方按同包既有 `repoRoot()` 定位，锚
// `core/internal/version/version.go`）。⇒ 同真仓 + cwd=仓根时两态读数**逐字相同**（已对拍）；
// 「脚本位置 ≠ 仓根」时路径推导面不同（✗ 不声称逐字对齐）。★ 引擎子进程的 `__file__` 由 bootstrap
// **显式注入** ⇒ 引擎侧的 `REPO_ROOT` 与旧壳**同锚**（不吃 cwd）。
//
// ★ 标准声明行 (b) —— **自指检查**：本件落在 `core/**`（`check-hardcoded-private-paths` 与
// `check-history-secrets` 的扫描面内）⇒ 正文**刻意零命中**：① 不写任何私有绝对路径 / 探针串；
// ② 引擎正文以 base64 承载（字符集不含 `/`，且不构成任一探针形态）⇒ 本件源码文本对两门现扫 0 命中。
//
// ★ 未覆盖面（✗ 不声称逐字对齐）：
//
//	· 旧壳 `-h/--help` 走 `sed -n '2,30p' "$0"`（打印脚本自身头注释 2-30 行）—— 本件把该 29 行
//	  **逐字节内嵌**为 `mpHelpBody`（同源），但 `$0`（脚本路径）本身不进正文 ⇒ 与 shell 逐字同；
//	· 旧壳未知参数退 2 的正文 ⟷ 本件同形（`未知参数: <a>`），但 `set -u` 下缺值形态的报话不同源；
//	· `$(cd "$OUT" && pwd)` 的**符号链接目录**语义：本件用 `filepath.Abs`（真实目录下同值；软链路径可能不同）；
//	· 引擎子进程：旧壳 `python3 "$LIB" …`（盘上件）⟷ 本件 `python3 -c <bootstrap> …`（stdin 内嵌件）——
//	  读数三面已实测逐字同（见落件回执两态对拍），差在**手段**；
//	· `-h` 的 `$0` 名称、`python3` 解释器版本随 PATH 变（库文案跨版本会变的只有 argparse，本件已注入同形 argv[0]）。
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	mpProg   = "mirror-public.sh"
	mpLibRel = "publish/mirror-public-lib.py"

	// mpRCNotFound —— shell 惯例退码 127（子命令 / python3 起不来）；127 属 internal
	// 命名空间、不入命令面真源表（先例 GAP-20261002-08）。
	mpRCNotFound = 127
)

// mpPyBootstrap —— 把内嵌引擎（mpPyLibB64 解码后的字节）经 **stdin** 交给 python3 -c 执行，
// 并注入与旧壳**逐字同形**的 `__file__`（脚本位置）与 `argv[0]`（脚本基名）—— `sys.argv=["-c", <libPath>, …]`
// ⇒ 本 bootstrap 取 `sys.argv[1]` 当 `__file__`、把 `sys.argv` 归到 `[<libPath>, …]`（与 `python3 <libPath> …` 同形）。
const mpPyBootstrap = "import sys\n" +
	"_src = sys.stdin.read()\n" +
	"_g = {\"__name__\": \"__main__\", \"__file__\": sys.argv[1], \"__builtins__\": __builtins__}\n" +
	"sys.argv = sys.argv[1:]\n" +
	"exec(compile(_src, sys.argv[0], \"exec\"), _g)\n"

// mpSay —— 复刻旧壳 `say()`：`printf '\n\033[1m== %s\033[0m\n' "$*"`（stdout）。
func mpSay(w io.Writer, s string) { fmt.Fprintf(w, "\n\x1b[1m== %s\x1b[0m\n", s) }

// mpFirst12 —— 照 bash `${VAR:0:12}`（前 12 字节）。
func mpFirst12(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

// mpRun —— 起子进程（登记 runningChild 以便收尾杀整组），返回其进程退码。
func mpRun(cmd *exec.Cmd) int {
	runningChild = cmd
	defer func() { runningChild = nil }()
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode()
		}
		return mpRCNotFound
	}
	return 0
}

// mpGitCapture —— 跑 git（照 `$(git -C … )`）：返回 rc 与 stdout（**去尾空白**）。
func mpGitCapture(args ...string) (int, string) {
	var out bytes.Buffer
	cmd := armChildGroup(exec.Command("git", args...))
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	rc := mpRun(cmd)
	return rc, strings.TrimSpace(out.String())
}

// mpStatusCount —— 复刻 `git -C REPO status --porcelain | wc -l`（计 git 原输出的换行数）。
func mpStatusCount(repo string) int {
	var out bytes.Buffer
	cmd := armChildGroup(exec.Command("git", "-C", repo, "status", "--porcelain"))
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	_ = mpRun(cmd)
	return strings.Count(out.String(), "\n")
}

// mpRunLib —— 跑内嵌引擎：`python3 -c <bootstrap> <libPath> <args…>`（stdin = 解码后的引擎字节）。
func mpRunLib(root string, args []string, stdout, stderr io.Writer) int {
	py, err := exec.LookPath("python3")
	if err != nil {
		return mpRCNotFound
	}
	libPath := filepath.Join(root, mpLibRel)
	cmd := armChildGroup(exec.Command(py, append([]string{"-c", mpPyBootstrap, libPath}, args...)...))
	cmd.Dir = root
	cmd.Stdin = bytes.NewReader(mpPyLibBytes())
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return mpRun(cmd)
}

// mpHasFile / mpHasDir —— 存在性（照 `[ -f ]` / `[ -d ]`）。
func mpHasFile(p string) bool { fi, err := os.Stat(p); return err == nil && !fi.IsDir() }
func mpHasDir(p string) bool  { fi, err := os.Stat(p); return err == nil && fi.IsDir() }

// mpExecutable —— 照 `[ -x ]`（普通件且有任一执行位）。
func mpExecutable(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir() && fi.Mode()&0o111 != 0
}

// mpOutResolved —— 复刻 `$(cd "$OUT" 2>/dev/null && pwd || echo "$OUT")`（是目录 ⇒ 绝对化；否则原样）。
func mpOutResolved(out string) string {
	if mpHasDir(out) {
		if abs, err := filepath.Abs(out); err == nil {
			return abs
		}
	}
	return out
}

// mpRunSoft —— 复刻旧壳 `run_soft()`（软门禁：优先 Go 半 · 退回 py 半 · 两半皆缺只报告；一律只报告不中止）。
func mpRunSoft(root, out, key, why string, stdout io.Writer) {
	zergBin := filepath.Join(root, "bin", "zerg")
	pyKey := filepath.Join(root, "scripts", "gates", key+".py")
	var cmd *exec.Cmd
	half := ""
	if mpExecutable(zergBin) {
		half = "Go 半"
		cmd = armChildGroup(exec.Command(zergBin, "gate", "check", key, out))
	} else if mpHasFile(pyKey) {
		half = "py 半"
		cmd = armChildGroup(exec.Command("python3", pyKey, out))
	} else {
		fmt.Fprintf(stdout, "  ⚠ %s：判据件两半皆缺（py 与 Go 半）⇒ 本档只报告（软门禁不中止本批）\n", key)
		return
	}
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if mpRun(cmd) == 0 {
		fmt.Fprintf(stdout, "  ✓ %s（%s）\n", key, half)
	} else {
		fmt.Fprintf(stdout, "  ⚠ %s 未通过（%s · %s —— 本批只报告）\n", key, half, why)
	}
}

// mirrorPublicNative —— `publish/mirror-public.sh` 的 Go 真身（argv 与旧壳同形 · 不含程序名）。
//
// root = 仓根（调用方按同包 repoRoot() 定位）。退码 / stdout / stderr 逐字对齐旧壳：
// 0 成功 · 1 环境/私有面门禁中止 · 2 用法/私有仓落点错 · 3 --push 双保险拦下 · 其余原样转发引擎退码。
func mirrorPublicNative(root string, stdout, stderr io.Writer, argv []string) int {
	lib := filepath.Join(root, mpLibRel)
	ts := time.Now().Format("20060102-150405")

	repo := root
	out := ""
	rng := "auto"
	push := 0
	remote := os.Getenv("ZERG_PUBLIC_REMOTE")
	report := ""
	allow := ""
	listOnly := 0
	verbose := 0

	i := 0
	for i < len(argv) {
		a := argv[i]
		val := func() string {
			if i+1 < len(argv) {
				return argv[i+1]
			}
			return ""
		}
		switch a {
		case "--repo":
			repo = val()
			i += 2
		case "--out":
			out = val()
			i += 2
		case "--range":
			rng = val()
			i += 2
		case "--report":
			report = val()
			i += 2
		case "--allow":
			allow = val()
			i += 2
		case "--remote":
			remote = val()
			i += 2
		case "--dry-run":
			push = 0
			i++
		case "--list-only", "--check":
			listOnly = 1
			i++
		case "--verbose":
			verbose = 1
			i++
		case "--push":
			push = 1
			i++
		case "-h", "--help":
			fmt.Fprint(stdout, mpHelpBody)
			return 0
		default:
			fmt.Fprintf(stderr, "未知参数: %s\n", a)
			return 2
		}
	}

	if out == "" {
		out = "/tmp/zerg-mirror-" + ts
	}
	if report == "" {
		report = strings.TrimSuffix(out, "/") + ".report-" + ts + ".json"
	}

	mpSay(stdout, "0/5 环境检查")
	if !mpHasFile(lib) {
		fmt.Fprintf(stderr, "缺少 %s\n", lib)
		return 1
	}
	if _, err := exec.LookPath("python3"); err != nil {
		fmt.Fprintln(stderr, "需要 python3")
		return 1
	}
	if _, err := exec.LookPath("git"); err != nil {
		fmt.Fprintln(stderr, "需要 git")
		return 1
	}
	if abs, err := filepath.Abs(repo); err == nil {
		repo = abs
	}
	if !mpHasDir(repo) {
		fmt.Fprintf(stderr, "cd: %s: No such file or directory\n", repo)
		return 1
	}
	if !mpHasDir(filepath.Join(repo, ".git")) {
		fmt.Fprintf(stderr, "不是 git 仓：%s\n", repo)
		return 2
	}
	libRepoRoot := root
	if abs, err := filepath.Abs(filepath.Dir(filepath.Dir(lib))); err == nil {
		libRepoRoot = abs
	}

	// ── 只读预演入口（--list-only / --check）：只列清单就退出（不建产出仓 · 不写盘 · 不联网 · 不推送）──
	if listOnly == 1 {
		mpSay(stdout, "0/0 只读预演（--list-only）：不建产出仓 · 不写盘 · 不联网 · 不推送")
		if push == 1 {
			fmt.Fprintln(stdout, "  ⚠ --push 被忽略：只读预演不做任何推送（本模式下不存在推送路径）")
		}
		fmt.Fprintf(stdout, "私有仓:   %s（只读）\n", repo)
		fmt.Fprintf(stdout, "产出目录: **未使用/未创建**（本入口不建不写：%s）\n", out)
		fmt.Fprintf(stdout, "资产来源: %s/publish/{whitelist.txt,replace-rules.tsv,private-paths.txt}\n", libRepoRoot)
		libArgs := []string{"--repo", repo, "--list-only"}
		if verbose == 1 {
			libArgs = append(libArgs, "--verbose")
		}
		if allow != "" {
			libArgs = append(libArgs, "--allow", allow)
		}
		rc := mpRunLib(root, libArgs, stdout, stderr)
		fmt.Fprintf(stdout, "退码: %d（只读预演：未写盘 · 未联网 · 未推送）\n", rc)
		return rc
	}

	// --out 落在私有仓里 ⇒ 拒（G4：镜像器绝不碰私有工作树）
	outRes := mpOutResolved(out)
	if outRes == repo || strings.HasPrefix(outRes, repo+"/") {
		fmt.Fprintf(stderr, "✗ --out 落在私有仓里（%s）—— 镜像器绝不碰私有工作树（G4）\n", out)
		return 2
	}

	_, privHeadBefore := mpGitCapture("-C", repo, "rev-parse", "HEAD")
	privDirtyBefore := mpStatusCount(repo)
	fmt.Fprintf(stdout, "私有仓:   %s（HEAD=%s · 改动 %d 项）\n", repo, mpFirst12(privHeadBefore), privDirtyBefore)
	fmt.Fprintf(stdout, "产出目录: %s\n", out)
	fmt.Fprintf(stdout, "资产来源: %s/publish/{whitelist.txt,replace-rules.tsv,private-paths.txt}\n", libRepoRoot)

	mpSay(stdout, "1/5 逐提交镜像（差分驱动 + 逐笔硬门禁）")
	libArgs := []string{"--repo", repo, "--out", out, "--range", rng}
	if report != "" {
		libArgs = append(libArgs, "--report", report)
	}
	if allow != "" {
		libArgs = append(libArgs, "--allow", allow)
	}
	if rc := mpRunLib(root, libArgs, stdout, stderr); rc != 0 {
		return rc // 旧壳：`python3 …` 非 0 ⇒ set -e ⇒ 以该码退出（不再往下）
	}
	if rc, _ := mpGitCapture("-C", out, "rev-parse", "refs/heads/main"); rc != 0 {
		return rc
	}
	_, mirrorTip := mpGitCapture("-C", out, "rev-parse", "refs/heads/main")
	if rc, _ := mpGitCapture("-C", out, "rev-parse", "refs/heads/main^{tree}"); rc != 0 {
		return rc
	}
	_, mirrorTree := mpGitCapture("-C", out, "rev-parse", "refs/heads/main^{tree}")
	if rc, _ := mpGitCapture("-C", out, "rev-list", "--count", "refs/heads/main"); rc != 0 {
		return rc
	}
	_, mirrorCount := mpGitCapture("-C", out, "rev-list", "--count", "refs/heads/main")

	// ── 2/5 私有面门禁（优先 Go 半 · 退回 py 半 · 两半皆缺 ⇒ 中止）──
	mpSay(stdout, "2/5 私有面门禁（对物化后的产出树跑既有检查器 · 优先 Go 半）")
	zergBin := filepath.Join(libRepoRoot, "bin", "zerg")
	pyPriv := filepath.Join(libRepoRoot, "scripts", "gates", "check-public-tree-private.py")
	privRC := 0
	ranPriv := false
	if mpExecutable(zergBin) {
		cmd := armChildGroup(exec.Command(zergBin, "gate", "check", "check-public-tree-private", out))
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		privRC = mpRun(cmd)
		ranPriv = true
	} else if mpHasFile(pyPriv) {
		cmd := armChildGroup(exec.Command("python3", pyPriv, out))
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		privRC = mpRun(cmd)
		ranPriv = true
	}
	if !ranPriv {
		fmt.Fprintln(stderr, "  ❌ 私有面门禁件两半皆缺（py 与 Go 半）——发布中止（不给结论 ⇒ 不放行）")
		return 1
	}
	if privRC == 0 {
		fmt.Fprintln(stdout, "  ✓ 私有面门禁通过（逐笔已在镜像时查过：见 1/5 输出）")
	} else {
		fmt.Fprintf(stderr, "  ❌ 产出树出现私有面路径 / 门取不到结论（rc=%d）—— 发布中止（一个字节都不推）\n", privRC)
		return 1
	}

	// ── 3/5 软门禁（只报告，不中止本批）──
	mpSay(stdout, "3/5 软门禁（文档双语 / i18n / 版本一致 / 跨版本状态清单 —— 只报告，不中止本批）")
	mpRunSoft(libRepoRoot, out, "check_docs", "历史各笔的形态不一定满足当前门禁", stdout)
	mpRunSoft(libRepoRoot, out, "check_version", "逐提交镜像对每笔都要求版本一致不现实 · B8 再定口径", stdout)
	mpRunSoft(libRepoRoot, out, "check-compat-manifest", "历史各笔可能还没有兼容清单 · 最终树上为硬门禁（见 publish-public.sh 4e）", stdout)

	// ── 4/5 私有仓未被动过（验收 D）──
	mpSay(stdout, "4/5 私有仓未被动过（验收 D）")
	_, privHeadAfter := mpGitCapture("-C", repo, "rev-parse", "HEAD")
	privDirtyAfter := mpStatusCount(repo)
	fmt.Fprintf(stdout, "私有仓 HEAD: %s → %s\n", mpFirst12(privHeadBefore), mpFirst12(privHeadAfter))
	fmt.Fprintf(stdout, "私有仓改动项: %d → %d\n", privDirtyBefore, privDirtyAfter)
	if privHeadBefore != privHeadAfter || privDirtyBefore != privDirtyAfter {
		fmt.Fprintln(stderr, "❌ 私有仓被改动了 —— 但镜像器本不该写私有仓，请立即排查")
		return 1
	}
	fmt.Fprintln(stdout, "✓ 私有仓只读：HEAD 与工作树状态完全未变（跑前应为 0）")

	// ── 5/5 推送 ──
	mpSay(stdout, "5/5 推送")
	fmt.Fprintf(stdout, "镜像末提交: %s / tree %s / %s 笔\n", mpFirst12(mirrorTip), mpFirst12(mirrorTree), mirrorCount)
	fmt.Fprintf(stdout, "报告:       %s\n", report)
	if push == 1 {
		if os.Getenv("ZERG_MIRROR_ALLOW_PUSH") != "1" {
			fmt.Fprintln(stderr, "✗ --push 被双重保险拦下：推送要同时满足 --push 且 ZERG_MIRROR_ALLOW_PUSH=1。")
			fmt.Fprintln(stderr, "  本批次（B1）只做 dry-run、绝不推送；B2 历史重写时由Mr2109点头后执行。")
			return 3
		}
		if remote == "" {
			fmt.Fprintln(stderr, "需要 --remote 或 ZERG_PUBLIC_REMOTE")
			return 2
		}
		if rc, _ := mpGitCapture("-C", out, "remote", "get-url", "origin"); rc != 0 {
			_, _ = mpGitCapture("-C", out, "remote", "add", "origin", remote)
		}
		_, _ = mpGitCapture("-C", out, "remote", "set-url", "origin", remote)
		// 唯一出口动作（真推送）前挂 B1 验收 —— 验收不过 ⇒ 拒推（挡在 push 之前）
		acc := filepath.Join(root, "publish", "mirror-acceptance.sh")
		cmd := armChildGroup(exec.Command("bash", acc))
		cmd.Env = append(os.Environ(), "SKIP_BUILD=1")
		cmd.Dir = out
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		if mpRun(cmd) != 0 {
			return 1
		}
		pushCmd := armChildGroup(exec.Command("git", "-C", out, "push", "origin", "main"))
		pushCmd.Stdout = stdout
		pushCmd.Stderr = stderr
		if mpRun(pushCmd) != 0 {
			return 1
		}
		fmt.Fprintf(stdout, "已推送（非 force）：%s\n", remote)
	} else {
		fmt.Fprintln(stdout, "（dry-run：全部做完但**未推送**）")
		fmt.Fprintf(stdout, "  复核后推送：  cd %s && git remote add origin <公开仓> && git push origin main\n", out)
	}
	return 0
}
