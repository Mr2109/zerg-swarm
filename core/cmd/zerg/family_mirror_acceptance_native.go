// family_mirror_acceptance_native.go —— `publish/mirror-acceptance.sh`（B1 验收套件 · 425 行）的 Go 真身
// （发布链 Go 半 · P6 批 · 与 `family_mirror_cn_native.go` 同族）。
//
// 病（原壳）：`publish/mirror-acceptance.sh` 是发布链上**唯一**「能失败」的验收套件（A–F 六格 + 反证格），
// 而它是两个推送出口的**前置闸**（`publish/mirror-public.sh` 的 `--push` 段与 `publish/mirror-cn.sh` 的
// ③ 段各 `SKIP_BUILD=1 bash …` 一次 · `GAP-20260927-138`）。它长在 shell 里 ⇒ CLI 面看不见、rc 档读历史
// 也只能手敲脚本。本件把六格编排搬进进程内，逐格与旧壳逐字对齐。
//
// ★ 判据单一真源（✗ 不造第二套）：
//
//	· 镜像动作 = **同一份** `mirrorPublicNative`（旧壳 `bash "$MIRROR" …`；那件本身在非推送档
//	  `exec bin/zerg publish mirror` ⇒ 同一份实现 · 本件跳过两层 exec）；
//	· 私有面门禁 = **同一件** `bin/zerg gate check check-public-tree-private`（子进程 · 只走 Go 半 ·
//	  py 半已退役 ⇒ 缺位即 BLOCKED，✗ 无 py 回退半）；
//	· 白名单谓词 / 读集 / 末树对拍 = **同一批 py 件**（`mirror-public-lib.py` 的 `_InlineHsec` /
//	  `mirror-verify-tree.py`）—— 数据面留 py，逐字同源（与镜像引擎同一条纪律）；
//	· JSON 排版（`--history` 与 rc 档以外的打印）= 旧壳的同一段 py 内联脚本（本件逐字搬）。
//
// ★ 退码（三档 · 向后兼容 · 与旧壳注释同）：
//
//	0 PASS / 1 FAIL / 2 BLOCKED（判据无牙 / 前置缺件 ⇒ **不发结论**）
//
// ★ 标准声明行 (a) —— **自指检查**（本件落 `core/**` ⇒ 在 `check-history-secrets` 与
// `check-hardcoded-private-paths` 的扫描面内）：夹具正文里的三种探针形态一律**拼接构造**
// （家目录 `/Users/…` · 内网地址 `192.168.x.x` · 假令牌 `ghp_…`），源码文本对两门现扫 0 命中 —— 先例
// `check_public_marks.go:1407`（同法拆串）。
//
// ★ 标准声明行 (b) —— **默认根定位差异**：旧壳 `REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"`
// （脚本位置）；Go 无 `__file__` 语境 ⇒ 从**入参 `root`** 取（调用方按同包 `repoRoot()` 定位）。
// 同真仓 + cwd=仓根时两态读数逐字相同；「脚本位置 ≠ 仓根」时不同（✗ 不声称逐字对齐）。
//
// ★ 未覆盖面（✗ 不声称逐字对齐）：
//
//	· `cd "$TOY"` 的**进程 cwd** 语义：旧壳切目录后所有相对写落 TOY 内（并因此被门71 的
//	  `cwgIsRepoLiteral` 判成「夹具仓内写」）；本件一律显式 `-C <dir>` / 绝对路径，**不切 cwd**
//	  ⇒ 侧效落点逐条相同，手段不同；
//	· `awk`/`comm`/`diff`/`sort` 四件小工具由本件内联实现（同语义：`substr($0,4)` 取状态后路径、
//	  `comm -13` 取「仅第二栏」）—— 语序与去重行为逐条对齐；
//	· `mkdir -p` / `cp -R` 的符号链接语义：本件按 `os.MkdirAll` + 逐件复制（真件复制，软链按目标内容读）
//	  —— 夹具树内无软链，读数同；
//	· 时间戳目录名 `zerg-mirror-accept-<ts>` 与 rc 档 `seq` 随运行变（✗ 逐字比 ⇒ 两态对拍按「掩时间戳」
//	  口径比，见落件回执）。
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	accProg    = "mirror-acceptance.sh"
	accMirRel  = "publish/mirror-public.sh"
	accTplRel  = pathMirrorLibPy // ★ 单一真源：路径常量只留 `paths_canonical.go` 一处（常量重复判据）
	accVerbRel = "publish/mirror-verify-tree.py"
	accWlRel   = pathPublishWhitelist // ★ 单一真源同 `accTplRel`
	accZergBin = "bin/zerg"
	accPrivGo  = "core/cmd/zerg/check_public_tree_private.go"
)

// accPyHistory —— 旧壳 `--history` 的 py 正文（逐字搬 · 单真源：按档内 `seq` 排，禁按 mtime）。
const accPyHistory = `
import json, sys
rows = []
with open(sys.argv[1], encoding="utf-8") as fh:
    for ln in fh:
        ln = ln.strip()
        if not ln:
            continue
        try:
            rows.append(json.loads(ln))
        except ValueError:
            pass
rows.sort(key=lambda r: r.get("seq", 0))      # ★ 按内容 seq 排，禁按 mtime
print("验收历史（%d 笔 · 按 seq 升序 · 禁按 mtime）：" % len(rows))
for r in rows:
    print("  #%s %s rc=%s %s pass=%s fail=%s blocked=%s  %s"
          % (r.get("seq"), r.get("ts"), r.get("rc"), r.get("verdict"),
             r.get("pass"), r.get("fail"), r.get("blocked"), r.get("workdir", "")))
`

// accPyReadSet —— 旧壳 D 段的读集 py 正文（逐字搬 · 复用 check-history-secrets 的 load_whitelist /
// published_path 单一真源；py 退场后走镜像器内化副本 `_InlineHsec`；两半皆缺 ⇒ rc=2）。
const accPyReadSet = `
import importlib.util, os, subprocess, sys

repo, out = sys.argv[1], sys.argv[2]
py = os.path.join(repo, "scripts", "gates", "check-history-secrets.py")
go_half = os.path.join(repo, "core", "cmd", "zerg", "check_history_secrets.go")
mirror_lib = os.path.join(repo, "publish", "mirror-public-lib.py")
def load(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod
if os.path.exists(py):
    m = load("chs", py)
elif os.path.exists(go_half) and os.path.exists(mirror_lib):
    m = load("chs_inl", mirror_lib)._InlineHsec
else:
    print("\u26d4 白名单/发布侧谓词两半皆缺（%s 与 Go 半 %s）——不给结论（rc=2）" % (py, go_half), file=sys.stderr)
    sys.exit(2)
entries = m.load_whitelist(os.path.join(repo, "publish", "whitelist.txt"))
files = subprocess.run(["git", "-C", repo, "-c", "core.quotepath=false", "ls-files"],
                       capture_output=True, text=True, check=True).stdout.splitlines()
sel = sorted(f for f in files if m.published_path(f, entries))
with open(out, "w", encoding="utf-8") as fh:
    fh.write("\n".join(sel) + ("\n" if sel else ""))
print("  读集(白名单选中): %d 条（本器会读它们 blob 的那一批）" % len(sel))
`

// accIO —— 一趟验收的运行面（工作区 / 两个输出流 / 三档计数）。
type accIO struct {
	root   string
	base   string
	stdout io.Writer
	stderr io.Writer
	pass   int
	fail   int
	blk    int
}

func (a *accIO) ok(f string, args ...any) {
	fmt.Fprintf(a.stdout, "  ✅ PASS  "+f+"\n", args...)
	a.pass++
}

func (a *accIO) bad(f string, args ...any) {
	fmt.Fprintf(a.stdout, "  ❌ FAIL  "+f+"\n", args...)
	a.fail++
}

// blocked —— 判据无牙 / 前置缺件 ⇒ 不发结论（rc=2）。
func (a *accIO) blocked(f string, args ...any) {
	fmt.Fprintf(a.stdout, "  ⛔ BLOCKED  "+f+"\n", args...)
	a.blk++
}

// head1 —— 复刻 `printf '\n\033[1m== %s\033[0m\n'`。
func (a *accIO) head1(s string) { fmt.Fprintf(a.stdout, "\n\x1b[1m== %s\x1b[0m\n", s) }

// git —— 跑 git 并把 stdout 收集为去尾空白的串（stderr 丢弃，同旧壳 `2>/dev/null` 的调用点）。
func (a *accIO) git(dir string, args ...string) (int, string) {
	return accGit(dir, args...)
}

// accGit —— `git -C <dir> …`：rc + 去尾空白 stdout（stderr 丢弃）。
func accGit(dir string, args ...string) (int, string) {
	full := args
	if dir != "" {
		full = append([]string{"-C", dir}, args...)
	}
	var out bytes.Buffer
	cmd := armChildGroup(exec.Command("git", full...))
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	rc := mpRun(cmd)
	return rc, strings.TrimSpace(out.String())
}

// accGitRaw —— `git -C <dir> …`：rc + **原始** stdout（不去尾空白 · 给 `wc -l` 口径用）。
func accGitRaw(dir string, args ...string) (int, string) {
	var out bytes.Buffer
	cmd := armChildGroup(exec.Command("git", append([]string{"-C", dir}, args...)...))
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	rc := mpRun(cmd)
	return rc, out.String()
}

// accWrite —— 写件（父目录按需建）。
func accWrite(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// accAppend —— `printf … >> <件>`。
func accAppend(path, content string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(content)
	return err
}

// accLineCount —— 复刻 `wc -l < <件>`（数换行数 · 缺件算 0）。
func accLineCount(path string) int {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	return strings.Count(string(b), "\n")
}

// accCopyTree —— 复刻 `cp -R <源> <目标>`（真件复制 · 无软链需求）。
func accCopyTree(src, dst string) error {
	return filepath.Walk(src, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if fi.IsDir() {
			return os.MkdirAll(target, fi.Mode().Perm()|0o700)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, fi.Mode().Perm())
	})
}

// accIndent —— 复刻 `sed 's/^/    /'`（逐行加前缀 · 保留行序）。
func accIndent(s, prefix string) string {
	if s == "" {
		return ""
	}
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i := range lines {
		lines[i] = prefix + lines[i]
	}
	return strings.Join(lines, "\n") + "\n"
}

// accGrepE —— 复刻 `grep -E '<pat>' <件>`（返回命中行原样）。
func accGrepE(path, pat string) []string {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	re, err := regexp.Compile(pat)
	if err != nil {
		return nil
	}
	var hits []string
	for _, ln := range strings.Split(string(b), "\n") {
		if re.MatchString(ln) {
			hits = append(hits, ln)
		}
	}
	return hits
}

// accGrepQ —— 复刻 `grep -q '<fixed>' <件>`。
func accGrepQ(path, fixed string) bool {
	b, err := os.ReadFile(path)
	return err == nil && strings.Contains(string(b), fixed)
}

// accRunPy —— 跑 python3（argv = 位置参数在 -c 之后）。
func accRunPy(script string, args []string, stdout, stderr io.Writer) int {
	py, err := exec.LookPath("python3")
	if err != nil {
		return mpRCNotFound
	}
	cmd := armChildGroup(exec.Command(py, append([]string{"-c", script}, args...)...))
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return mpRun(cmd)
}

// mirrorTo —— `bash "$MIRROR" <argv…> ><log> 2>&1`：本件直调 `mirrorPublicNative`（同一份实现）。
func (a *accIO) mirrorTo(logPath string, argv ...string) int {
	f, err := os.Create(logPath)
	if err != nil {
		return 1
	}
	defer f.Close()
	return mirrorPublicNative(a.root, f, f, argv)
}

// privCheck —— 旧壳 `priv_check`：只走 Go 半；Go 半缺位 ⇒ rc=2（不给结论）。
func (a *accIO) privCheck(tree string, stdout, stderr io.Writer) int {
	bin := filepath.Join(a.root, accZergBin)
	if fi, err := os.Stat(bin); err == nil && !fi.IsDir() && fi.Mode()&0o111 != 0 {
		cmd := armChildGroup(exec.Command(bin, "gate", "check", "check-public-tree-private", tree))
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		return mpRun(cmd)
	}
	fmt.Fprintln(stderr, "⛔ 私有面门禁件 Go 半缺位（py 半已退役）——不给结论（rc=2）")
	return 2
}

// accPrivOut —— 旧壳 `priv_check <树> | sed 's/^/    /'`：**只**把 stdout 缩排（✗ 不动 stderr）。
//
// ★ 逐字对齐面：旧壳那条管道没有 `2>&1` ⇒ 检查器的 stderr 原样落到终端（不缩排），
// 只有 stdout 过 `sed`。故本件把 stderr 直接接 `a.stderr`，只缩排 stdout。
func (a *accIO) privOut(tree string) string {
	var buf bytes.Buffer
	a.privCheck(tree, &buf, a.stderr)
	return buf.String()
}

// accDiffCmd —— 旧壳 `diff <A> <B> | sed 's/^/      /'`：**真调同一个 `diff`**（✗ 自造 diff 格式）。
//
// 为什么必须调外部件而不是打印占位：`diff` 的正文形态（`2c2` / `<` / `---` / `>`）是它自己的出口，
// 逐字复刻等于另造一套判据；同壳同件才是同源。取不到 `diff` ⇒ 静默（旧壳 `|| true` 同语义）。
func accDiffCmd(a, b string) string {
	out, err := exec.Command("diff", a, b).Output()
	if len(out) == 0 && err != nil {
		// `diff` 报「不同」时 rc=1 但仍有正文；真取不到正文（缺件/起不来）⇒ 空
		if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 1 {
			return ""
		}
	}
	return accIndent(string(out), "      ")
}

// ── 夹具探针（★ 拼接构造：源码里不出现连成一串的私径 / 内网地址 / 令牌形态）───────────────
var (
	accProbeHome = "/Users/" + "ms" + "01" // 家目录路径探针
	accProbeLAN  = "192.168." + "1" + ".5" // 内网地址探针
	// ★ 假令牌探针（**非真凭据** · 只为让 B 格能失败）：逐字节等于旧壳 :174 那一串，
	//   但**拆成两段拼接** ⇒ 本件源码文本对 `ghp_[A-Za-z0-9]{20,}` 现扫 0 命中（不必再往
	//   `publish/mirror-allow.txt` 加一条豁免 · 先例 `check_public_marks.go:1407`）。
	accProbeTok = "ghp_" + "AAAABBBBCCCCDDDDEEEEFFFFGGGGHHHHIIII"
)

// mirrorAcceptanceNative —— `publish/mirror-acceptance.sh` 的 Go 真身（argv 与旧壳同形 · 不含程序名）。
func mirrorAcceptanceNative(root string, stdout, stderr io.Writer, argv []string) int {
	stateDir := os.Getenv("MIRROR_ACCEPT_STATE_DIR")
	if stateDir == "" {
		zd := os.Getenv("ZERG_STATE_DIR")
		if zd == "" {
			zd = filepath.Join(os.Getenv("HOME"), ".zerg", "state")
		}
		stateDir = filepath.Join(zd, "mirror-acceptance")
	}
	rcFile := filepath.Join(stateDir, "rc.jsonl")

	// ── --history：只读打印历史（按 seq 升序），不跑验收、不建工作区、不写盘 ──
	if len(argv) > 0 && argv[0] == "--history" {
		if fi, err := os.Stat(rcFile); err != nil || fi.Size() == 0 {
			fmt.Fprintf(stdout, "无 rc 档：%s\n", rcFile)
			fmt.Fprintln(stdout, "  ⚠「无档/无目录」不说明任何事 —— 只说明此处从未落过 rc 档（**禁**据此判「没跑过」）")
			return 0
		}
		return accRunPy(accPyHistory, []string{rcFile}, stdout, stderr)
	}

	mirror := filepath.Join(root, accMirRel)
	// 前置缺件 ⇒ BLOCKED（不发结论），不是「验收失败」。
	if _, err := os.Stat(mirror); err != nil {
		fmt.Fprintf(stderr, "⛔ BLOCKED 前置缺件：%s —— 验收不成立，不发结论（rc=2）\n", mirror)
		return 2
	}
	// 私有面门禁件：只认 Go 半（bin/zerg + core/cmd/zerg/check_public_tree_private.go）。
	binOK := false
	if fi, err := os.Stat(filepath.Join(root, accZergBin)); err == nil && !fi.IsDir() && fi.Mode()&0o111 != 0 {
		binOK = true
	}
	if !binOK || !mpHasFile(filepath.Join(root, accPrivGo)) {
		fmt.Fprintln(stderr, "⛔ BLOCKED 前置缺件：私有面门禁件 Go 半缺位（py 半已退役）—— 验收不成立，不发结论（rc=2）")
		return 2
	}

	a := &accIO{root: root, stdout: stdout, stderr: stderr}
	a.base = "/tmp/zerg-mirror-accept-" + time.Now().Format("20060102-150405")
	toy := filepath.Join(a.base, "toy-clean")
	leak := filepath.Join(a.base, "toy-leak")
	if err := os.MkdirAll(a.base, 0o755); err != nil {
		fmt.Fprintf(stderr, "⛔ BLOCKED 建不了工作区：%s\n", a.base)
		return 2
	}

	fmt.Fprintf(stdout, "验收工作区: %s\n", a.base)
	fmt.Fprintf(stdout, "私有仓:     %s（只读）\n", root)
	_, privHead0 := a.git(root, "rev-parse", "HEAD")
	// 2026-09-22（2.5.11 第 5 项）：D 的判据口径 = HEAD + **本工具读过的路径**（`core.quotepath=false`）。
	// ★ 此处要**原始** stdout（`wc -l` 数的是换行数，不能被 TrimSpace 吃掉空档）。
	status0 := filepath.Join(a.base, "priv-status0.txt")
	_, raw0 := accGitRaw(root, "-c", "core.quotepath=false", "status", "--porcelain")
	_ = accWrite(status0, raw0)
	privDirty0 := strings.Count(raw0, "\n")

	// ── 自造一次性小仓（干净版）────────────────────────────────────────
	if err := buildToyClean(toy); err != nil {
		fmt.Fprintf(stderr, "⛔ BLOCKED 夹具仓建不起来：%v\n", err)
		return 2
	}
	_, toyNStr := a.git(toy, "rev-list", "--count", "HEAD")

	// ── 自造一次性小仓（**中间那笔**埋假令牌）──────────────────────────
	if err := buildToyLeak(leak); err != nil {
		fmt.Fprintf(stderr, "⛔ BLOCKED 夹具仓建不起来：%v\n", err)
		return 2
	}
	_, revs := a.git(leak, "rev-list", "--reverse", "HEAD")
	leakMid := accNth(revs, 3)

	a.head1("A 幂等：同一区间跑两次（末提交 tree 相同 + 提交数相同）")
	// A.1 小仓
	for _, d := range []string{"a1", "a2"} {
		log := filepath.Join(a.base, "toy-"+d+".log")
		if rc := a.mirrorTo(log, "--repo", toy, "--out", filepath.Join(a.base, "toy-"+d), "--range", "HEAD"); rc != 0 {
			a.bad("A(小仓) 第 %s 次运行失败，见 %s", d, log)
		}
	}
	_, t1 := a.git(filepath.Join(a.base, "toy-a1"), "rev-parse", "refs/heads/main^{tree}")
	_, t2 := a.git(filepath.Join(a.base, "toy-a2"), "rev-parse", "refs/heads/main^{tree}")
	_, n1 := a.git(filepath.Join(a.base, "toy-a1"), "rev-list", "--count", "refs/heads/main")
	_, n2 := a.git(filepath.Join(a.base, "toy-a2"), "rev-list", "--count", "refs/heads/main")
	fmt.Fprintf(stdout, "  小仓末提交 tree: %s / %s · 提交数: %s / %s（私有 %s 笔）\n", t1, t2, n1, n2, toyNStr)
	if t1 != "" && t1 == t2 {
		a.ok("A(小仓) tree hash 完全相同：%s", accCut(t1, 12))
	} else {
		a.bad("A(小仓) tree hash 不同（%s vs %s）", t1, t2)
	}
	if n1 == n2 && n1 == toyNStr {
		a.ok("A(小仓) 提交数相同且与私有仓一致：%s", n1)
	} else {
		a.bad("A(小仓) 提交数不同/不一致（%s vs %s，私有 %s）", n1, n2, toyNStr)
	}
	_, c1 := a.git(filepath.Join(a.base, "toy-a1"), "rev-list", "--all", "--format=%H %T %P")
	_, c2 := a.git(filepath.Join(a.base, "toy-a2"), "rev-list", "--all", "--format=%H %T %P")
	if c1 == c2 {
		a.ok("A(小仓) 逐笔提交哈希/tree/父链完全相同")
	} else {
		a.bad("A(小仓) 提交链不一致")
	}

	// A.3 增量（.mirror-state ⇒ 默认区间 = last-mirrored..HEAD）：只补新一笔，父链接上
	_ = accAppend(filepath.Join(toy, "scripts", "start.sh"), "\n// 增量\n")
	_, _ = a.git(toy, "commit", "-qam", "c5: 只加一笔")
	_, incPriv := a.git(toy, "rev-parse", "HEAD")
	incLog := filepath.Join(a.base, "toy-inc.log")
	if rc := a.mirrorTo(incLog, "--repo", toy, "--out", filepath.Join(a.base, "toy-a1")); rc != 0 {
		a.bad("A(增量) 运行失败")
	}
	if accGrepQ(incLog, "增量（.mirror-state") && accGrepQ(incLog, "待镜像:  1 笔") {
		a.ok("A(增量) 默认区间自动解析为 last-mirrored..HEAD，只镜像 1 笔")
	} else {
		a.bad("A(增量) 未走增量区间")
	}
	a1Dir := filepath.Join(a.base, "toy-a1")
	_, incN := a.git(a1Dir, "rev-list", "--count", "refs/heads/main")
	_, incT := a.git(a1Dir, "log", "-1", "--format=%(trailers:key=GitOrigin-RevId,valueonly)")
	_, incP := a.git(a1Dir, "log", "-1", "--format=%P")
	_, incPrev := a.git(a1Dir, "rev-parse", "refs/heads/main~1")
	if incN == "5" && incT == incPriv && incP == incPrev {
		a.ok("A(增量) 提交数 5 · 末笔 trailer=%s · 父链接在上一笔镜像提交上", accCut(incPriv, 10))
	} else {
		a.bad("A(增量) 结果异常（count=%s trailer=%s parent=%s）", incN, accCut(incT, 10), accCut(incP, 10))
	}

	// A.2 真仓：最早 3 笔（含最早一笔）
	_, logAll := a.git(root, "log", "--reverse", "--format=%H")
	third := accNth(logAll, 3)
	for _, d := range []string{"r1", "r2"} {
		log := filepath.Join(a.base, "real-"+d+".log")
		mrc := a.mirrorTo(log, "--repo", root, "--out", filepath.Join(a.base, "real-"+d), "--range", third)
		if mrc == 0 {
			// 通过
		} else if accGrepQ(log, "空转：公开树 docs/ 下扫到 0 个文件") && mpHasDir(filepath.Join(a.base, "real-"+d, ".git")) {
			fmt.Fprintf(stdout, "  ℹ A(真仓) %s：镜像器因「早期提交无 docs/ ⇒ 正向断言空转」中止（rc=%d · **预期**）—— 产出树已建成，幂等仍可比\n", d, mrc)
		} else {
			a.bad("A(真仓) 第 %s 次运行失败（rc=%d，**非空转原因**），见 %s", d, mrc, log)
		}
	}
	_, rt1 := a.git(filepath.Join(a.base, "real-r1"), "rev-parse", "refs/heads/main^{tree}")
	_, rt2 := a.git(filepath.Join(a.base, "real-r2"), "rev-parse", "refs/heads/main^{tree}")
	_, rn1 := a.git(filepath.Join(a.base, "real-r1"), "rev-list", "--count", "refs/heads/main")
	_, rn2 := a.git(filepath.Join(a.base, "real-r2"), "rev-list", "--count", "refs/heads/main")
	fmt.Fprintf(stdout, "  真仓末提交 tree: %s / %s · 提交数: %s / %s\n", rt1, rt2, rn1, rn2)
	if rt1 != "" && rt1 == rt2 {
		a.ok("A(真仓 3 笔) tree hash 完全相同：%s", accCut(rt1, 12))
	} else {
		a.bad("A(真仓) tree hash 不同")
	}
	if rn1 == rn2 && rn1 == "3" {
		a.ok("A(真仓) 提交数相同＝3")
	} else {
		a.bad("A(真仓) 提交数 %s vs %s", rn1, rn2)
	}

	a.head1("B 逐提交硬门禁：中间那笔埋假令牌 ⇒ 整批中止 + 产出零提交")
	leakLog := filepath.Join(a.base, "leak.log")
	leakRC := a.mirrorTo(leakLog, "--repo", leak,
		"--out", filepath.Join(a.base, "leak-out"),
		"--report", filepath.Join(a.base, "leak-report.json"))
	fmt.Fprintf(stdout, "  埋串提交: %s（第 3/4 笔，**不是 tip**）· 退出码: %d\n", accCut(leakMid, 12), leakRC)
	if hits := accGrepE(leakLog, "敏感扫描命中|私有提交|零提交"); len(hits) > 0 {
		fmt.Fprint(stdout, accIndent(strings.Join(hits, "\n"), "    "))
	}
	if leakRC != 0 {
		a.ok("B 整批中止（退出码 %d ≠ 0）", leakRC)
	} else {
		a.bad("B 竟然没有中止（rc=0）")
	}
	midHit, tipHit := 0, 0
	if accGrepQ(leakLog, accCut(leakMid, 10)) {
		midHit = 1
	}
	if accGrepQ(leakLog, "第 3/4 笔") {
		midHit = 1
	}
	_, leakTip := a.git(leak, "rev-parse", "HEAD")
	for _, ln := range accGrepE(leakLog, "敏感扫描命中|镜像路径") {
		if strings.Contains(ln, accCut(leakTip, 10)) {
			tipHit = 1
		}
	}
	if midHit == 1 && tipHit == 0 {
		a.ok("B 命中定位到**中间那笔** %s（第 3/4 笔；tip 未被扫到 ⇒ 不是只扫 tip）", accCut(leakMid, 10))
	} else {
		a.bad("B 未定位到中间提交（mid_hit=%d tip_hit=%d）", midHit, tipHit)
	}
	leakOut := filepath.Join(a.base, "leak-out")
	if _, err := os.Stat(leakOut); err != nil {
		a.ok("B 产出目录不存在 ⇒ 零提交、零对象")
	} else if _, cnt := a.git(leakOut, "rev-list", "--all", "--count"); cnt == "0" {
		a.ok("B 产出目录存在但可达提交数=0 ⇒ 零提交")
	} else {
		a.bad("B 产出目录里有提交（应为零）")
	}

	// B2（同族）：私有面**路径**门禁也能中止（白名单临时加一条 docs/项目文档/ 的副本）
	wl, _ := os.ReadFile(filepath.Join(root, accWlRel))
	wlBad := filepath.Join(a.base, "wl-bad.txt")
	badLines := []string{}
	for _, ln := range strings.Split(string(wl), "\n") {
		if ln == "shared/" {
			badLines = append(badLines, "shared/", "docs/项目文档/")
			continue
		}
		badLines = append(badLines, ln)
	}
	_ = accWrite(wlBad, strings.Join(badLines, "\n"))
	if !regexp.MustCompile(`(?m)^docs/项目文档/$`).MatchString(string(mustRead(wlBad))) {
		a.bad("B2 临时白名单构造失败")
	}
	faceLog := filepath.Join(a.base, "face.log")
	faceRC := accMirrorLib(root, faceLog, []string{
		"--repo", toy, "--out", filepath.Join(a.base, "face-out"),
		"--range", "HEAD", "--whitelist", wlBad,
	})
	if hits := accGrepE(faceLog, "私有面门禁|← 规则|零提交"); len(hits) > 0 {
		fmt.Fprint(stdout, accIndent(strings.Join(hits, "\n"), "    "))
	}
	if faceRC != 0 && !mpHasDir(filepath.Join(a.base, "face-out")) {
		a.ok("B2 私有面路径门禁同样中止且产出零提交")
	} else {
		a.bad("B2 私有面门禁未生效（rc=%d）", faceRC)
	}

	a.head1("C 可构建：真仓 3 笔（含最早一笔）镜像 → worktree → go build ./...")
	buildOK := 1
	_, revList := a.git(filepath.Join(a.base, "real-r1"), "rev-list", "--reverse", "refs/heads/main")
	i := 0
	for _, sha := range strings.Split(revList, "\n") {
		if sha == "" {
			continue
		}
		i++
		_, origin := a.git(filepath.Join(a.base, "real-r1"), "log", "-1", "--format=%(trailers:key=GitOrigin-RevId,valueonly)", sha)
		fmt.Fprintf(stdout, "  镜像提交 %d: %s（私有来源 %s）\n", i, accCut(sha, 12), accCut(origin, 12))
		wt := filepath.Join(a.base, "wt-"+strconv.Itoa(i))
		if rc, _ := a.git(filepath.Join(a.base, "real-r1"), "worktree", "add", "-q", "--detach", wt, sha); rc != 0 {
			a.bad("C worktree add 失败")
			buildOK = 0
			continue
		}
		for _, m := range []string{"core", "agent"} {
			if !mpHasFile(filepath.Join(wt, m, "go.mod")) {
				fmt.Fprintf(stdout, "    – [%s/] 该提交无 go.mod（跳过）\n", m)
				continue
			}
			if os.Getenv("SKIP_BUILD") == "1" {
				fmt.Fprintf(stdout, "    … [%s/] SKIP_BUILD=1\n", m)
				continue
			}
			out, ok := accGoBuild(filepath.Join(wt, m))
			if ok {
				fmt.Fprintf(stdout, "    ✅ go build ./... [%s/] 通过\n", m)
			} else {
				fmt.Fprintf(stdout, "    ❌ go build ./... [%s/] 失败：\n", m)
				fmt.Fprint(stdout, accIndent(accHeadLines(out, 6), "        "))
				buildOK = 0
			}
		}
	}
	if buildOK == 1 {
		a.ok("C 3 笔镜像（含最早一笔）core/ 与 agent/ 全部构建通过")
	} else {
		a.bad("C 有提交/模块构建未过（见上，历史遗留即如实记录）")
	}

	a.head1("D 不碰私有仓：HEAD 不变 + **本工具读过的路径**跑前跑后一致")
	_, privHead1 := a.git(root, "rev-parse", "HEAD")
	status1 := filepath.Join(a.base, "priv-status1.txt")
	_, raw1 := accGitRaw(root, "-c", "core.quotepath=false", "status", "--porcelain")
	_ = accWrite(status1, raw1)
	privDirty1 := strings.Count(raw1, "\n")
	fmt.Fprintf(stdout, "  HEAD:             %s → %s\n", accCut(privHead0, 12), accCut(privHead1, 12))
	fmt.Fprintf(stdout, "  全仓脏件(INFO):   %d → %d（**只报不判** —— 别的会话在改件不算本器的账）\n", privDirty0, privDirty1)
	if privHead0 == privHead1 {
		a.ok("D 私有仓 HEAD 未变")
	} else {
		a.bad("D 私有仓 HEAD 被改！")
	}

	// 读集 = 白名单选中的仓内路径（复用单一真源 check-history-secrets 的解析器）。
	readset := filepath.Join(a.base, "priv-readset.txt")
	// ★ 旧壳此格的 py 失败**只**透它自己的报话（`⛔ 白名单/发布侧谓词两半皆缺…` + rc=2），
	//   壳不再补一句 ⇒ 本件同样不补（多打一句就是新增输出面）。
	_ = accRunPy(accPyReadSet, []string{root, readset}, stdout, stderr)
	if fi, err := os.Stat(readset); err != nil || fi.Size() == 0 {
		a.blocked("D 读集是空的 ⇒ 判据无牙（穷举器选不中任何件）：不发结论（BLOCKED rc=2，禁当绿）")
	}
	rsSet := map[string]bool{}
	for _, ln := range strings.Split(strings.TrimRight(string(mustRead(readset)), "\n"), "\n") {
		if ln != "" {
			rsSet[ln] = true
		}
	}
	// 只判读集里的件：跑前干净、跑后变脏（或反之）⇒ 红。逐档算「读集里的脏件」集合。
	dirtyCount := make([]string, 2)
	for t, raw := range []string{raw0, raw1} {
		var picked []string
		for _, ln := range strings.Split(strings.TrimRight(raw, "\n"), "\n") {
			if len(ln) < 4 {
				continue
			}
			path := ln[3:] // awk substr($0,4)
			if rsSet[path] {
				picked = append(picked, path)
			}
		}
		sort.Strings(picked)
		picked = accUniq(picked)
		_ = accWrite(filepath.Join(a.base, "priv-readdirty"+strconv.Itoa(t)+".txt"),
			strings.Join(picked, "\n")+accNL(picked))
		dirtyCount[t] = strconv.Itoa(len(picked))
	}
	fmt.Fprintf(stdout, "  读集里的脏件:     %s → %s\n", dirtyCount[0], dirtyCount[1])
	rd0, _ := os.ReadFile(filepath.Join(a.base, "priv-readdirty0.txt"))
	rd1, _ := os.ReadFile(filepath.Join(a.base, "priv-readdirty1.txt"))
	if bytes.Equal(rd0, rd1) {
		a.ok("D 读集（本工具读过的路径）里的脏件集合未变")
	} else {
		a.bad("D 读集里有件被动了（跑前跑后集合不同 —— 这才是真动了私有仓）")
		fmt.Fprint(stdout, accDiffCmd(filepath.Join(a.base, "priv-readdirty0.txt"),
			filepath.Join(a.base, "priv-readdirty1.txt")))
	}
	// 读集**之外**的变化：只报不判。
	newLines := accCommOnlyB(strings.Split(raw0, "\n"), strings.Split(raw1, "\n"))
	if len(newLines) == 0 {
		fmt.Fprintln(stdout, "  ℹ 全仓脏件集合也没变")
	} else {
		fmt.Fprintf(stdout, "  ℹ 跑动期间全仓新增 %d 条脏件（读集外 ⇒ 外部会话所致 · **不记本器的账**）\n", len(newLines))
		for _, ln := range newLines {
			fmt.Fprintf(stdout, "      · %s\n", ln)
		}
	}

	a.head1("E 私有面排除：产出树 + 逐笔历史")
	if a.privCheck(filepath.Join(a.base, "toy-a1"), io.Discard, io.Discard) == 0 {
		a.ok("E 私有面门禁（Go 半 / py）对产出树 rc=0（产出树 = 夹具仓 toy-a1）")
	} else {
		a.bad("E 检查器在产出树上报红 / 门两半皆缺")
		fmt.Fprint(stdout, accIndent(a.privOut(filepath.Join(a.base, "toy-a1")), "    "))
	}
	hits := 0
	pats := []string{"docs/项目文档", "docs/01-设计", "AGENTS.md", "^bin/", "scripts/build/publish-public.sh",
		"scripts/gates/check-history-secrets.py", "scripts/gates/check-public-tree-private.py",
		"gateway/fleet.yaml", "gateway/deploy_x3.sh", "^publish/"}
	_, eRevs := a.git(filepath.Join(a.base, "real-r1"), "rev-list", "refs/heads/main")
	for _, sha := range strings.Split(eRevs, "\n") {
		if sha == "" {
			continue
		}
		_, tree := a.git(filepath.Join(a.base, "real-r1"), "-c", "core.quotepath=false", "ls-tree", "-r", "--name-only", sha)
		names := strings.Split(tree, "\n")
		for _, pat := range pats {
			re := regexp.MustCompile(pat)
			n := 0
			for _, nm := range names {
				if re.MatchString(nm) {
					n++
				}
			}
			if n != 0 {
				fmt.Fprintf(stdout, "    ❌ %s 命中 %s × %d\n", accCut(sha, 8), pat, n)
				hits++
			}
		}
	}
	if hits == 0 {
		a.ok("E E 清单 10 类路径在**每一笔**镜像提交里 0 命中")
	} else {
		a.bad("E 命中 %d 处", hits)
	}
	// E 反证：污染一棵产出树 ⇒ 检查器必须报红（证明这道门能失败）
	poison := filepath.Join(a.base, "poison")
	_ = os.RemoveAll(poison)
	_ = accCopyTree(filepath.Join(a.base, "real-r1"), poison)
	_ = os.RemoveAll(filepath.Join(poison, ".git"))
	for _, d := range []string{"docs/项目文档", "bin"} {
		_ = os.MkdirAll(filepath.Join(poison, d), 0o755)
	}
	for _, f := range []string{"docs/项目文档/内部.md", "AGENTS.md", "bin/zerg-core", "gateway/deploy_x3.sh"} {
		_ = accWrite(filepath.Join(poison, f), "x\n")
	}
	poisonRC := a.privCheck(poison, io.Discard, io.Discard)
	if poisonRC == 1 {
		a.ok("E 反证有效：污染树被检查器报红（rc=1）")
	} else {
		a.bad("E 反证失败（应 rc=1，实为 %d）", poisonRC)
	}

	a.head1("F（额外）末树一致性：差分累加的末树 == 直接导出（白名单+规则）")
	verb := filepath.Join(root, accVerbRel)
	if out, rc := accCapture(armChildGroup(exec.Command("python3", verb, "--repo", root,
		"--mirror", filepath.Join(a.base, "real-r1"), "--commit", third))); rc == 0 {
		fmt.Fprint(stdout, accIndent(out, "  "))
		a.ok("F 3 笔镜像末树与直接导出逐字节相同")
	} else {
		fmt.Fprint(stdout, accIndent(out, "  "))
		a.bad("F 末树与直接导出不一致")
	}
	other := accNth(logAll, 8)
	_, negRC := accCapture(armChildGroup(exec.Command("python3", verb, "--repo", root,
		"--mirror", filepath.Join(a.base, "real-r1"), "--commit", other)))
	if negRC == 1 {
		a.ok("F 反证有效：拿别的私有提交去比 ⇒ 报不一致（rc=1）")
	} else {
		a.bad("F 反证失败（应 rc=1，实为 %d）", negRC)
	}

	// ── 小结 + 持久 rc 档（§④ 落持久目录、不落 /tmp；§⑤ 带 seq，读历史按 seq 不按 mtime）──
	a.head1("小结")
	rc := 0
	verdict := "PASS"
	switch {
	case a.blk != 0:
		rc, verdict = 2, "BLOCKED"
	case a.fail != 0:
		rc, verdict = 1, "FAIL"
	}
	fmt.Fprintf(stdout, "  通过 %d 项 · 失败 %d 项 · 阻塞 %d 项 · 工作区 %s\n", a.pass, a.fail, a.blk, a.base)
	skipBuild := os.Getenv("SKIP_BUILD")
	if skipBuild == "" {
		skipBuild = "0"
	}
	if err := os.MkdirAll(stateDir, 0o755); err == nil {
		seq := accLineCount(rcFile) + 1
		host, _ := os.Hostname()
		line := fmt.Sprintf(`{"seq":%d,"ts":"%s","verdict":"%s","rc":%d,"pass":%d,"fail":%d,"blocked":%d,"workdir":"%s","skip_build":"%s","host":"%s"}`,
			seq, time.Now().UTC().Format("2006-01-02T15:04:05Z"), verdict, rc, a.pass, a.fail, a.blk, a.base, skipBuild, host)
		if err := accAppend(rcFile, line+"\n"); err != nil {
			fmt.Fprintln(stdout, "  ⚠ rc 档写入失败（本次读数仍在本行，不退码）")
		}
		fmt.Fprintf(stdout, "  ↳ rc 档（持久 · 非 /tmp）：%s 第 %d 笔 → rc=%d %s\n", rcFile, seq, rc, verdict)
	} else {
		fmt.Fprintf(stdout, "  ⚠ rc 档目录不可写：%s（本行即读数；不退码）\n", stateDir)
	}
	switch rc {
	case 2:
		fmt.Fprintln(stdout, "  ⛔ 有阻塞项（判据无牙 / 前置缺件）——**不发结论**，逐条见上")
	case 1:
		fmt.Fprintln(stdout, "  ❌ 有失败项，逐条见上")
	default:
		fmt.Fprintln(stdout, "  ✅ A–E（+F 额外）全部通过（可失败项均已在失败路径上验过：B 埋串、E 污染树、F 错提交）")
	}
	return rc
}

// buildToyClean —— 旧壳 `mkdir -p "$TOY" && cd "$TOY" && git init -q -b main` 起四笔的干净夹具仓。
func buildToyClean(toy string) error {
	if err := os.MkdirAll(toy, 0o755); err != nil {
		return err
	}
	if rc, _ := accGit("", "init", "-q", "-b", "main", toy); rc != 0 {
		return fmt.Errorf("git init 失败：%s", toy)
	}
	_, _ = accGit(toy, "config", "user.email", "toy@example.com")
	_, _ = accGit(toy, "config", "user.name", "Toy Dev")
	for _, d := range []string{"core", "ui", "scripts", "shared/resources", "docs/项目文档", "docs/01-设计", "gateway"} {
		if err := os.MkdirAll(filepath.Join(toy, d), 0o755); err != nil {
			return err
		}
	}
	files := map[string]string{
		"core/main.go": "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"hello\") }\n",
		"ui/Cargo.toml": "[package]\nname = \"zerg-ui\"\nversion = \"0.1.0\"\n\n[features]\n" +
			"default = [\"zerg-roundtable\"]\nzerg-roundtable = [\"dep:zerg-roundtable\"]\n\n[dependencies]\n" +
			"zerg-roundtable = { path = \"../zerg-cocoon\", optional = true }\n",
		"scripts/run.sh":  "#!/bin/bash\n# repo at " + accProbeHome + "/work · Mr2109 says hi · worker " + accProbeLAN + "\necho run\n",
		"docs/项目文档/x.md":  "# 私密文档\n",
		"docs/01-设计/y.md": "# 私有设计\n",
		// ★ 夹具件名必须对得上既有映射目标（`publish/docs/<任意目录>/X` ⇒ `docs/X`）⇒ 用真名 CLI.zh-CN.md。
		"publish/docs/CLI.zh-CN.md": "# CLI（夹具 · 供私有面门正向断言）\n",
		"AGENTS.md":                 "# agents\n",
		"gateway/deploy_x3.sh":      "deploy\n",
	}
	for rel, content := range files {
		if err := accWrite(filepath.Join(toy, rel), content); err != nil {
			return err
		}
	}
	return buildToyCommits(toy, [][2]string{
		{"c1: init", ""},
		{"c2: add util, drop deploy_x3", ""},
		{"c3: rename run.sh -> start.sh, add shared", ""},
		{"c4: touch main.go", ""},
	})
}

// buildToyCommits —— 干净夹具仓的四笔（c1 已在文件铺好后；c2–c4 各带自己的改动）。
func buildToyCommits(toy string, plan [][2]string) error {
	// c1：铺好的全量
	if rc, _ := accGit(toy, "add", "-A"); rc != 0 {
		return fmt.Errorf("git add 失败")
	}
	if rc, _ := accGit(toy, "commit", "-qm", plan[0][0]); rc != 0 {
		return fmt.Errorf("git commit 失败：%s", plan[0][0])
	}
	// c2：加 core/util.go · 删 gateway/deploy_x3.sh
	if err := accWrite(filepath.Join(toy, "core", "util.go"), "package main\n\nfunc util() int { return 42 }\n"); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(toy, "gateway", "deploy_x3.sh"))
	if rc, _ := accGit(toy, "add", "-A"); rc != 0 {
		return fmt.Errorf("git add 失败")
	}
	if rc, _ := accGit(toy, "commit", "-qm", plan[1][0]); rc != 0 {
		return fmt.Errorf("git commit 失败：%s", plan[1][0])
	}
	// c3：git mv run.sh → start.sh · 加 shared/resources/x.go
	if rc, _ := accGit(toy, "mv", "scripts/run.sh", "scripts/start.sh"); rc != 0 {
		return fmt.Errorf("git mv 失败")
	}
	if err := accWrite(filepath.Join(toy, "shared", "resources", "x.go"), "package resources\n"); err != nil {
		return err
	}
	if rc, _ := accGit(toy, "add", "-A"); rc != 0 {
		return fmt.Errorf("git add 失败")
	}
	if rc, _ := accGit(toy, "commit", "-qm", plan[2][0]); rc != 0 {
		return fmt.Errorf("git commit 失败：%s", plan[2][0])
	}
	// c4：追加 main.go
	if err := accAppend(filepath.Join(toy, "core", "main.go"), "\n// touched\n"); err != nil {
		return err
	}
	if rc, _ := accGit(toy, "add", "-A"); rc != 0 {
		return fmt.Errorf("git add 失败")
	}
	if rc, _ := accGit(toy, "commit", "-qm", plan[3][0]); rc != 0 {
		return fmt.Errorf("git commit 失败：%s", plan[3][0])
	}
	return nil
}

// buildToyLeak —— 旧壳第二座夹具仓：四笔，**第 3 笔**（不是 tip）埋假令牌。
func buildToyLeak(leak string) error {
	if err := os.MkdirAll(leak, 0o755); err != nil {
		return err
	}
	if rc, _ := accGit("", "init", "-q", "-b", "main", leak); rc != 0 {
		return fmt.Errorf("git init 失败：%s", leak)
	}
	_, _ = accGit(leak, "config", "user.email", "toy@example.com")
	_, _ = accGit(leak, "config", "user.name", "Toy Dev")
	if err := os.MkdirAll(filepath.Join(leak, "core"), 0o755); err != nil {
		return err
	}
	steps := []struct{ file, content, msg string }{
		{"core/a.go", "package main\n", "c1: 干净"},
		{"core/b.go", "package main // util\n", "c2: 干净"},
		{"core/leak.go", "package main\n\nvar token = \"" + accProbeTok + "\"\n", "c3: 中间提交，埋假令牌"},
		{"core/c.go", "package main // after\n", "c4: tip 干净"},
	}
	for _, s := range steps {
		if err := accWrite(filepath.Join(leak, s.file), s.content); err != nil {
			return err
		}
		if rc, _ := accGit(leak, "add", "-A"); rc != 0 {
			return fmt.Errorf("git add 失败")
		}
		if rc, _ := accGit(leak, "commit", "-qm", s.msg); rc != 0 {
			return fmt.Errorf("git commit 失败：%s", s.msg)
		}
	}
	return nil
}

// accMirrorLib —— 直接跑**引擎**（旧壳 B2 格：`python3 mirror-public-lib.py …` · 不经镜像器壳）。
func accMirrorLib(root, logPath string, args []string) int {
	f, err := os.Create(logPath)
	if err != nil {
		return 1
	}
	defer f.Close()
	cmd := armChildGroup(exec.Command("python3", append([]string{filepath.Join(root, accTplRel)}, args...)...))
	cmd.Stdout = f
	cmd.Stderr = f
	return mpRun(cmd)
}

// accCapture —— 跑子进程并收 combined output（旧壳 `$(… 2>&1)`）。
func accCapture(cmd *exec.Cmd) (string, int) {
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	rc := mpRun(cmd)
	return buf.String(), rc
}

// accGoBuild —— `cd <目录> && GOFLAGS=-mod=mod GOPROXY=off go build ./...`（combined output）。
func accGoBuild(dir string) (string, bool) {
	cmd := armChildGroup(exec.Command("go", "build", "./..."))
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOPROXY=off")
	out, rc := accCapture(cmd)
	return out, rc == 0
}

// accNth —— 复刻 `sed -n <N>p`（1-based · 越界回空）。
func accNth(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if n >= 1 && n <= len(lines) {
		return lines[n-1]
	}
	return ""
}

// accCut —— 复刻 bash `${VAR:0:N}`（前 N 字节）。
func accCut(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// accHeadLines —— 复刻 `head -N`。
func accHeadLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n") + "\n"
}

// accUniq —— 复刻 `sort -u` 后的去重（入参须已排序）。
func accUniq(in []string) []string {
	out := in[:0]
	var prev string
	for i, s := range in {
		if i > 0 && s == prev {
			continue
		}
		out = append(out, s)
		prev = s
	}
	return out
}

// accNL —— 排序结果非空时补尾换行（`printf '%s\n'` 行为）。
func accNL(in []string) string {
	if len(in) == 0 {
		return ""
	}
	return "\n"
}

// accCommOnlyB —— 复刻 `comm -13 <A> <B>`（A/B 两档先各排序；取「仅 B 有」的那些行）。
func accCommOnlyB(a, b []string) []string {
	as := append([]string{}, a...)
	bs := append([]string{}, b...)
	sort.Strings(as)
	sort.Strings(bs)
	inA := map[string]bool{}
	for _, s := range as {
		inA[s] = true
	}
	seen := map[string]bool{}
	var out []string
	for _, s := range bs {
		if s == "" || inA[s] || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// mustRead —— 读件（缺件回空 · 夹具用）。
func mustRead(p string) []byte {
	b, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	return b
}

// accWithSkipBuild —— 旧壳 `SKIP_BUILD=1 bash publish/mirror-acceptance.sh` 的等价物：
// 临时置位 `SKIP_BUILD=1` 跑完即复原（✗ 不外泄到本进程后续调用 —— 进程内跑没有子进程 env 边界，
// 必须自己建一条）。
func accWithSkipBuild(fn func() int) int {
	const k = "SKIP_BUILD"
	old, had := os.LookupEnv(k)
	_ = os.Setenv(k, "1")
	defer func() {
		if had {
			_ = os.Setenv(k, old)
			return
		}
		_ = os.Unsetenv(k)
	}()
	return fn()
}
