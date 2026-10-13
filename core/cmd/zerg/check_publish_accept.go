// check_publish_accept.go —— 发布验收三条（验-2 / 验-3 / 验-4）的 **真门**（判据件）。
//
// 病（承接项 ①-6）：这三条原先是 `zerg publish accept`（`family_publish_accept_native.go`）
// 件内的**薄适配器**（`paccRunMirrorTip` / `paccRunFsck` / `paccRunRollbackDigest`）—— 只在
// 编排件内部跑、`zerg gate check <名>` **打不到**、无独立自检、不进判据登记面 ⇒ 与「一切按 CLI
// 标准走」不一致（同件里另五条走的都是既有门 / CLI 真身）。
//
// 治法（已拍）：把三条**升格成真门** —— 落成本件三个判据臂入口（进 `gateCheckGoEntry`，
// `zerg gate check <名>` 直接打到），各带 `--self-test`；同批把 `zerg publish accept` 的步表
// 由 `paccKindAdapter`（件内直判）改指 `paccKindGate`（**单一判据真源**：✗ 不两套并存判两次）。
//
// 判据逐字来源 = 任务单 ㈤v2（发布链收口 8 条验收 · `…/acceptance-prep/20261011-㈤v2.md`）：
//
//	· check-publish-accept-lsremote —— 验-2：镜像意图 tip == 远端 tip ∧ refs 差集空（**先断言两侧非空** · 防空过）
//	· check-publish-accept-fsck     —— 验-3：`git fsck --full` rc=0 ∧ `^(dangling|unreachable|missing|broken|error)` 计数==0
//	· check-publish-accept-rollback —— 验-4：现值 `shasum -a 256` == 记录值 `退点归档-<日期>/<件>.sha256`（空==空假 PASS 必红）
//
// 入口签名（与 inproc 表逐字同形）：`func(stdout, stderr io.Writer, argv []string) int`；argv 不含程序名。
// 退码三档：`0` 通过 / `1` 判负（红）/ `2` 不给结论（缺件 / 用法错 · fail-closed —— ✗ 缺件≠绿）。
//
// ★ 实参面：`--mirror <目录>` / `--remote <名|路径>` / `--rollback <件>` / `--root <根>` /
//
//	`--state-dir <目录>`（自检/夹具专用旋钮 · 缺省 = `devEditArchiveDir()` 的父目录口径）/ `--self-test` / `-h`。
//	三条门**不读任何环境变量**（✗ 不新造隐式旋钮面）—— 编排件侧把既有 env 旋钮展开成显式实参。
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
	"strings"
	"time"
)

const (
	cpaccLsremoteKey = "check-publish-accept-lsremote" // 验-2
	cpaccFsckKey     = "check-publish-accept-fsck"     // 验-3
	cpaccRollbackKey = "check-publish-accept-rollback" // 验-4

	cpaccRCOK      = 0   // 通过
	cpaccRCFail    = 1   // 判负（红）
	cpaccRCBlocked = 2   // 不给结论（缺件 / 用法错 · fail-closed）
	cpaccSpawnFail = 127 // shell 惯例码：外部子进程起不来（非 ExitError）

	cpaccDefaultRemote = "public-gh"
	cpaccArchivePrefix = "退点归档-"
	cpaccDateLayout    = "20060102"
)

var cpaccDanglingRe = regexp.MustCompile(`(?m)^(dangling|unreachable|missing|broken|error)`)

// ── 实参面（三条门共用 · 各自忽略无关位）────────────────────────────────────

type cpaccOpts struct {
	mirror   string
	remote   string
	rollback string
	root     string
	stateDir string
	selfTest bool
	help     bool
}

// cpaccParse —— 解析 argv（不含程序名）。ok=false ⇒ 调用方直接返回 rc（用法错）。
func cpaccParse(prog string, argv []string, out io.Writer) (cpaccOpts, int, bool) {
	o := cpaccOpts{remote: cpaccDefaultRemote}
	needVal := func(i *int, name string) (string, bool) {
		if *i+1 >= len(argv) {
			fmt.Fprintf(out, "%s: `%s` 缺值 ⇒ 不给结论（rc=%d）\n", prog, name, cpaccRCBlocked)
			return "", false
		}
		*i = *i + 1
		return argv[*i], true
	}
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "--mirror":
			v, ok := needVal(&i, "--mirror")
			if !ok {
				return o, cpaccRCBlocked, false
			}
			o.mirror = v
		case strings.HasPrefix(a, "--mirror="):
			o.mirror = strings.TrimPrefix(a, "--mirror=")
		case a == "--remote":
			v, ok := needVal(&i, "--remote")
			if !ok {
				return o, cpaccRCBlocked, false
			}
			o.remote = v
		case strings.HasPrefix(a, "--remote="):
			o.remote = strings.TrimPrefix(a, "--remote=")
		case a == "--rollback":
			v, ok := needVal(&i, "--rollback")
			if !ok {
				return o, cpaccRCBlocked, false
			}
			o.rollback = v
		case strings.HasPrefix(a, "--rollback="):
			o.rollback = strings.TrimPrefix(a, "--rollback=")
		case a == "--root":
			v, ok := needVal(&i, "--root")
			if !ok {
				return o, cpaccRCBlocked, false
			}
			o.root = v
		case strings.HasPrefix(a, "--root="):
			o.root = strings.TrimPrefix(a, "--root=")
		case a == "--state-dir":
			v, ok := needVal(&i, "--state-dir")
			if !ok {
				return o, cpaccRCBlocked, false
			}
			o.stateDir = v
		case strings.HasPrefix(a, "--state-dir="):
			o.stateDir = strings.TrimPrefix(a, "--state-dir=")
		case a == "--self-test":
			o.selfTest = true
		case a == "-h" || a == "--help":
			o.help = true
		default:
			fmt.Fprintf(out, "%s: 未知旗标 %s ⇒ 不给结论（rc=%d）\n", prog, a, cpaccRCBlocked)
			return o, cpaccRCBlocked, false
		}
	}
	return o, cpaccRCOK, true
}

// cpaccUsage —— 三条门共用的用法串（人面）。
func cpaccUsage(prog string, w io.Writer) {
	fmt.Fprintf(w, "用法：zerg gate check %s [--mirror <镜像产出目录>] [--remote <远端名|路径>] "+
		"[--rollback <回退演练件>] [--root <仓根>] [--state-dir <状态目录>] [--self-test]\n", prog)
}

// cpaccMain —— 三条门的**共用入口体**（解析 ⇒ 自检 / 执行）。
func cpaccMain(prog string, stdout, stderr io.Writer, argv []string,
	run func(root string, o cpaccOpts, out io.Writer) int, selfTest func(out io.Writer) int) int {
	o, rc, ok := cpaccParse(prog, argv, stderr)
	if !ok {
		return rc
	}
	if o.help {
		cpaccUsage(prog, stdout)
		return cpaccRCOK
	}
	if o.selfTest {
		return selfTest(stdout)
	}
	root := strings.TrimSpace(o.root)
	if root == "" {
		root = repoRoot()
	}
	if strings.TrimSpace(root) == "" {
		fmt.Fprintf(stderr, "%s: 解析不到仓根（`--root` 未给且 `repoRoot()` 取不到）⇒ 不给结论（rc=%d）\n",
			prog, cpaccRCBlocked)
		return cpaccRCBlocked
	}
	return run(root, o, stdout)
}

// ── 三个判据臂入口（`gateCheckGoEntry` 的值）────────────────────────────────

func checkPublishAcceptLsremoteGateMain(stdout, stderr io.Writer, argv []string) int {
	return cpaccMain(cpaccLsremoteKey, stdout, stderr, argv, cpaccRunLsremote, cpaccSelfTestLsremote)
}

func checkPublishAcceptFsckGateMain(stdout, stderr io.Writer, argv []string) int {
	return cpaccMain(cpaccFsckKey, stdout, stderr, argv, cpaccRunFsck, cpaccSelfTestFsck)
}

func checkPublishAcceptRollbackGateMain(stdout, stderr io.Writer, argv []string) int {
	return cpaccMain(cpaccRollbackKey, stdout, stderr, argv, cpaccRunRollback, cpaccSelfTestRollback)
}

// ── 验-2：镜像意图 tip == 远端 tip ∧ refs 差集空（先断言两侧非空）──────────────

func cpaccRunLsremote(root string, o cpaccOpts, out io.Writer) int {
	img := strings.TrimSpace(o.mirror)
	if img == "" {
		fmt.Fprintf(out, "   ⛔ 验-2 缺件：未给镜像产出目录（--mirror）⇒ 跑不动 ⇒ fail-closed（缺件≠绿）\n")
		return cpaccRCBlocked
	}
	if !cpaccIsDir(img) {
		fmt.Fprintf(out, "   ⛔ 验-2 缺件：镜像产出目录不在盘（%s）⇒ fail-closed\n", img)
		return cpaccRCBlocked
	}
	remote := strings.TrimSpace(o.remote)
	if remote == "" {
		remote = cpaccDefaultRemote
	}
	r1, intent := cpaccGitOut(img, "rev-parse", "refs/heads/main")
	if r1 != 0 || intent == "" {
		fmt.Fprintln(out, "   ✗ 验-2 红：镜像意图 tip 取空")
		return cpaccRCFail
	}
	r2, rem := cpaccRemoteTip(root, remote)
	if r2 != 0 || rem == "" {
		fmt.Fprintln(out, "   ✗ 验-2 红：远端 tip 取空")
		return cpaccRCFail
	}
	if intent != rem {
		fmt.Fprintf(out, "   ✗ 验-2 红：意图 tip(%s) ≠ 远端 tip(%s)\n", cpaccShort(intent), cpaccShort(rem))
		return cpaccRCFail
	}
	lrefs := cpaccShowRefs(img)
	rrefs := cpaccRemoteRefs(root, remote)
	if lrefs == "" || rrefs == "" {
		fmt.Fprintln(out, "   ✗ 验-2 红：refs 面一侧为空（空==空）")
		return cpaccRCFail
	}
	if d := cpaccComm3Count(lrefs, rrefs); d != 0 {
		fmt.Fprintf(out, "   ✗ 验-2 红：refs 差集非空（%d 行）\n", d)
		return cpaccRCFail
	}
	fmt.Fprintln(out, "   ✅ 验-2 意图 tip==远端 tip ∧ refs 差集空")
	return cpaccRCOK
}

// ── 验-3：`git fsck --full` rc=0 ∧ dangling/unreachable/missing/broken/error 计数==0 ──

func cpaccRunFsck(root string, o cpaccOpts, out io.Writer) int {
	_ = root
	tree := strings.TrimSpace(o.mirror)
	if tree == "" || !cpaccIsDir(tree) {
		fmt.Fprintf(out, "   ⛔ 验-3 缺件：正式镜像树不在盘（%s）⇒ fail-closed\n", tree)
		return cpaccRCBlocked
	}
	if rc, _ := cpaccGitOut(tree, "rev-parse", "--git-dir"); rc != 0 {
		fmt.Fprintf(out, "   ⛔ 验-3 缺件：%s 不是 git 仓（切之后才有对象）⇒ fail-closed\n", tree)
		return cpaccRCBlocked
	}
	rc, log := cpaccGitOutFull(tree, "fsck", "--full")
	if rc != 0 {
		fmt.Fprintf(out, "   ✗ 验-3 红：fsck rc=%d\n", rc)
		cpaccTail(out, log, 4, "   ")
		return cpaccRCFail
	}
	n := len(cpaccDanglingRe.FindAllString(log, -1))
	if n != 0 {
		fmt.Fprintf(out, "   ✗ 验-3 红：有 dangling/悬空/缺失对象 %d 行\n", n)
		cpaccTail(out, cpaccDanglingMatching(log), 4, "   ")
		return cpaccRCFail
	}
	fmt.Fprintln(out, "   ✅ 验-3 fsck rc=0 ∧ 悬空/缺失计数 0")
	return cpaccRCOK
}

// ── 验-4：现值 sha256 == 记录值（`退点归档-<日期>/<件>.sha256` · 退-1 写）────────

func cpaccRunRollback(root string, o cpaccOpts, out io.Writer) int {
	fileRel := strings.TrimSpace(o.rollback)
	if fileRel == "" {
		fmt.Fprintf(out, "   ⛔ 验-4 缺件：未给回退演练件（--rollback）⇒ 跑不动 ⇒ fail-closed\n")
		return cpaccRCBlocked
	}
	archiveDir := devEditArchiveDir()
	if sd := strings.TrimSpace(o.stateDir); sd != "" {
		archiveDir = filepath.Join(sd, cpaccArchivePrefix+time.Now().Format(cpaccDateLayout))
	}
	rec := filepath.Join(archiveDir, devEditSafeName(fileRel)+".sha256")
	if fi, err := os.Stat(rec); err != nil || fi.Size() == 0 {
		fmt.Fprintf(out, "   ✗ 验-4 红：记录值缺位（空==空假 PASS）（%s）\n", rec)
		return cpaccRCFail
	}
	now := cpaccSha256File(filepath.Join(root, fileRel))
	if now == "" {
		now = cpaccSha256File(fileRel)
	}
	recv := cpaccFirstField(rec)
	if now == "" || recv == "" || now != recv {
		fmt.Fprintf(out, "   ✗ 验-4 红：现值(%s) ≠ 记录值(%s) ⇒ 演练 FAIL（不给结论）\n",
			cpaccShort(now), cpaccShort(recv))
		return cpaccRCFail
	}
	fmt.Fprintln(out, "   ✅ 验-4 现值 == 记录值")
	return cpaccRCOK
}

// ── 底层取数助手（git / shasum · 只读）──────────────────────────────────────

func cpaccGitOut(dir string, args ...string) (int, string) {
	a := append([]string{"-C", dir}, args...)
	return cpaccRunCmd("", "git", a...)
}

// cpaccGitOutFull —— 同 cpaccGitOut，但**保留 stderr + stdout 合并**（fsck 的悬空行走 stderr）。
func cpaccGitOutFull(dir string, args ...string) (int, string) {
	a := append([]string{"-C", dir}, args...)
	cmd := armChildGroup(exec.Command("git", a...))
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	runningChild = cmd
	defer func() { runningChild = nil }()
	err := cmd.Run()
	return cpaccExitCode(err), buf.String()
}

// cpaccRunCmd —— 跑一条命令；返回 (rc, stdout·trim)。rc=127 ⇒ 起不来（非 ExitError）。
func cpaccRunCmd(dir, name string, args ...string) (int, string) {
	cmd := armChildGroup(exec.Command(name, args...))
	if dir != "" {
		cmd.Dir = dir
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	runningChild = cmd
	defer func() { runningChild = nil }()
	err := cmd.Run()
	return cpaccExitCode(err), strings.TrimSpace(out.String())
}

func cpaccRemoteTip(root, remote string) (int, string) {
	rc, out := cpaccGitOut(root, "ls-remote", remote, "refs/heads/main")
	if out == "" {
		return rc, ""
	}
	return rc, strings.TrimSpace(strings.Fields(out)[0])
}

func cpaccShowRefs(dir string) string {
	_, out := cpaccGitOut(dir, "show-ref")
	return cpaccSortedRefNames(out)
}

func cpaccRemoteRefs(root, remote string) string {
	_, out := cpaccGitOut(root, "ls-remote", "--refs", remote)
	return cpaccSortedRefNames(out)
}

// cpaccSortedRefNames —— 取 `git show-ref`/`ls-remote` 输出的**第 2 列（ref 名）**、排序、拼行（照 ㈤v2 的 awk+sort）。
func cpaccSortedRefNames(raw string) string {
	names := []string{}
	for _, ln := range strings.Split(raw, "\n") {
		f := strings.Fields(ln)
		if len(f) >= 2 {
			names = append(names, f[1])
		}
	}
	sort.Strings(names)
	return strings.Join(names, "\n")
}

// cpaccComm3Count —— 两侧 ref 名（已排序 · 换行分隔）的对称差行数（照 ㈤v2 的 `comm -3 … | wc -l`）。
func cpaccComm3Count(l, r string) int {
	inR := map[string]bool{}
	for _, s := range strings.Split(r, "\n") {
		if s != "" {
			inR[s] = true
		}
	}
	inL := map[string]bool{}
	for _, s := range strings.Split(l, "\n") {
		if s != "" {
			inL[s] = true
		}
	}
	n := 0
	for _, s := range strings.Split(l, "\n") {
		if s != "" && !inR[s] {
			n++
		}
	}
	for _, s := range strings.Split(r, "\n") {
		if s != "" && !inL[s] {
			n++
		}
	}
	return n
}

func cpaccSha256File(path string) string {
	rc, out := cpaccRunCmd("", "shasum", "-a", "256", path)
	if rc != 0 {
		return ""
	}
	return cpaccFirstFieldText(out)
}

func cpaccFirstField(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return cpaccFirstFieldText(string(b))
}

func cpaccFirstFieldText(t string) string {
	f := strings.Fields(strings.TrimSpace(t))
	if len(f) == 0 {
		return ""
	}
	return f[0]
}

func cpaccExitCode(err error) int {
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	return cpaccSpawnFail
}

func cpaccIsDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func cpaccShort(s string) string {
	if s == "" {
		return "—"
	}
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

// cpaccTail —— 复刻 `tail -n <n>`（取合并输出末 n 行 · 逐行加前缀）。
func cpaccTail(w io.Writer, s string, n int, prefix string) {
	if s == "" {
		return
	}
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	for _, ln := range lines {
		fmt.Fprintf(w, "%s%s\n", prefix, ln)
	}
}

func cpaccDanglingMatching(log string) string {
	var b strings.Builder
	for _, m := range cpaccDanglingRe.FindAllString(log, -1) {
		b.WriteString(m)
		b.WriteString("\n")
	}
	return b.String()
}

// ── 自检（各三/四格 · 成对负控 · 只在仓外临时目录造件并自回收）──────────────────
//
// ★ 末行格式照同族（`check-cli-ownership` 的 F2 族）：`自检：N 格过 · M 格不过`
//   —— `zerg gate self-test-summary` 的末行解析器据此读出「格 / 失败」。

// cpaccSelfTestRC —— 自检聚合退码：全过 ⇒ 0；有不过 ⇒ 2（fail-closed：判据自检坏了 = 判不了）。
func cpaccSelfTestRC(bad int) int {
	if bad == 0 {
		return cpaccRCOK
	}
	return cpaccRCBlocked
}

// cpaccHaveGit —— 自检夹具需要 git；不在盘 ⇒ 记一格不过（✗ 不静默跳过）。
func cpaccHaveGit(report func(string, bool, string)) bool {
	if _, err := exec.LookPath("git"); err != nil {
		report("前置：git 在 PATH", false, "git 不在 PATH ⇒ 自检夹具造不出")
		return false
	}
	return true
}

// cpaccSeedMirror —— 造一枚带 `refs/heads/main` 的「镜像」仓（仓外临时目录）。
func cpaccSeedMirror(dir string) bool {
	if rc, _ := cpaccRunCmd("", "git", "init", "-q", dir); rc != 0 {
		return false
	}
	_, _ = cpaccRunCmd("", "git", "-C", dir, "config", "user.email", "t@t.local")
	_, _ = cpaccRunCmd("", "git", "-C", dir, "config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hi\n"), 0o644); err != nil {
		return false
	}
	if rc, _ := cpaccRunCmd("", "git", "-C", dir, "add", "-A"); rc != 0 {
		return false
	}
	if rc, _ := cpaccRunCmd("", "git", "-C", dir, "commit", "-qm", "init"); rc != 0 {
		return false
	}
	_, _ = cpaccRunCmd("", "git", "-C", dir, "branch", "-M", "main")
	return true
}

func cpaccMkTemp(prefix string) string {
	d, err := os.MkdirTemp("", prefix)
	if err != nil {
		return ""
	}
	return d
}

func cpaccSelfTestLsremote(stdout io.Writer) int {
	ok, bad := 0, 0
	report := func(name string, cond bool, detail string) {
		if cond {
			fmt.Fprintf(stdout, "   ✓ %s\n", name)
			ok++
			return
		}
		fmt.Fprintf(stdout, "   ✗ %s：%s\n", name, detail)
		bad++
	}

	// 甲 缺件负控：无 --mirror ⇒ rc=2（缺件≠绿）。
	{
		var b bytes.Buffer
		rc := cpaccRunLsremote("", cpaccOpts{remote: cpaccDefaultRemote}, &b)
		report("甲 缺 --mirror ⇒ 不给结论（rc=2）",
			rc == cpaccRCBlocked && strings.Contains(b.String(), "缺件"),
			fmt.Sprintf("期望 rc=2 且报「缺件」，得 rc=%d out=%q", rc, b.String()))
	}
	if !cpaccHaveGit(report) {
		fmt.Fprintf(stdout, "   自检：%d 格过 · %d 格不过（仓外临时目录 · 未碰真仓）\n", ok, bad)
		return cpaccSelfTestRC(bad)
	}
	A, B := cpaccMkTemp("cpacc-lsr-a-"), cpaccMkTemp("cpacc-lsr-b-")
	if A != "" {
		defer os.RemoveAll(A)
	}
	if B != "" {
		defer os.RemoveAll(B)
	}
	if A == "" || B == "" || !cpaccSeedMirror(A) {
		report("前置：夹具仓造得出", false, "镜像仓 / 临时目录造不出")
		fmt.Fprintf(stdout, "   自检：%d 格过 · %d 格不过（仓外临时目录 · 未碰真仓）\n", ok, bad)
		return cpaccSelfTestRC(bad)
	}
	// 远端 = A 的裸克隆（只余 refs/heads/main）。
	if rc, _ := cpaccRunCmd("", "git", "clone", "-q", "--bare", A, B); rc != 0 {
		report("前置：夹具远端仓造得出", false, "`git clone --bare` 失败")
		fmt.Fprintf(stdout, "   自检：%d 格过 · %d 格不过（仓外临时目录 · 未碰真仓）\n", ok, bad)
		return cpaccSelfTestRC(bad)
	}

	// 乙 正控：镜像 ⟷ 远端 同 tip 同 refs ⇒ rc=0。
	{
		var b bytes.Buffer
		rc := cpaccRunLsremote(A, cpaccOpts{mirror: A, remote: B}, &b)
		report("乙 正控 同 tip 同 refs ⇒ rc=0",
			rc == cpaccRCOK, fmt.Sprintf("期望 rc=0，得 rc=%d out=%q", rc, b.String()))
	}
	// 丙 负控：远端多一条 ref ⇒ refs 差集非空 ⇒ rc=1。
	{
		_, _ = cpaccRunCmd("", "git", "-C", B, "branch", "extra")
		var b bytes.Buffer
		rc := cpaccRunLsremote(A, cpaccOpts{mirror: A, remote: B}, &b)
		report("丙 负控 远端多一条 ref ⇒ 差集非空 ⇒ rc=1",
			rc == cpaccRCFail && strings.Contains(b.String(), "差集非空"),
			fmt.Sprintf("期望 rc=1 且报「差集非空」，得 rc=%d out=%q", rc, b.String()))
	}
	fmt.Fprintf(stdout, "   自检：%d 格过 · %d 格不过（仓外临时目录 · 未碰真仓）\n", ok, bad)
	return cpaccSelfTestRC(bad)
}

func cpaccSelfTestFsck(stdout io.Writer) int {
	ok, bad := 0, 0
	report := func(name string, cond bool, detail string) {
		if cond {
			fmt.Fprintf(stdout, "   ✓ %s\n", name)
			ok++
			return
		}
		fmt.Fprintf(stdout, "   ✗ %s：%s\n", name, detail)
		bad++
	}

	// 甲 缺件负控：无 --mirror ⇒ rc=2。
	{
		var b bytes.Buffer
		rc := cpaccRunFsck("", cpaccOpts{}, &b)
		report("甲 缺 --mirror ⇒ 不给结论（rc=2）",
			rc == cpaccRCBlocked && strings.Contains(b.String(), "缺件"),
			fmt.Sprintf("期望 rc=2 且报「缺件」，得 rc=%d out=%q", rc, b.String()))
	}
	if !cpaccHaveGit(report) {
		fmt.Fprintf(stdout, "   自检：%d 格过 · %d 格不过（仓外临时目录 · 未碰真仓）\n", ok, bad)
		return cpaccSelfTestRC(bad)
	}
	repo := cpaccMkTemp("cpacc-fsck-")
	if repo != "" {
		defer os.RemoveAll(repo)
	}
	if repo == "" || !cpaccSeedMirror(repo) {
		report("前置：夹具仓造得出", false, "临时仓造不出")
		fmt.Fprintf(stdout, "   自检：%d 格过 · %d 格不过（仓外临时目录 · 未碰真仓）\n", ok, bad)
		return cpaccSelfTestRC(bad)
	}
	// 乙 正控：干净仓 ⇒ rc=0。
	{
		var b bytes.Buffer
		rc := cpaccRunFsck("", cpaccOpts{mirror: repo}, &b)
		report("乙 正控 干净仓 ⇒ rc=0",
			rc == cpaccRCOK, fmt.Sprintf("期望 rc=0，得 rc=%d out=%q", rc, b.String()))
	}
	// 丙 负控：造 1 个 dangling blob ⇒ rc=1（补抓 dangling）。
	{
		cmd := armChildGroup(exec.Command("git", "-C", repo, "hash-object", "-w", "--stdin"))
		cmd.Stdin = bytes.NewBufferString("dangling\n")
		var b bytes.Buffer
		cmd.Stdout = &b
		cmd.Stderr = io.Discard
		runningChild = cmd
		_ = cmd.Run()
		runningChild = nil
		var ob bytes.Buffer
		rc := cpaccRunFsck("", cpaccOpts{mirror: repo}, &ob)
		report("丙 负控 有 dangling ⇒ rc=1",
			rc == cpaccRCFail && strings.Contains(ob.String(), "dangling"),
			fmt.Sprintf("期望 rc=1 且报「dangling」，得 rc=%d out=%q", rc, ob.String()))
	}
	fmt.Fprintf(stdout, "   自检：%d 格过 · %d 格不过（仓外临时目录 · 未碰真仓）\n", ok, bad)
	return cpaccSelfTestRC(bad)
}

func cpaccSelfTestRollback(stdout io.Writer) int {
	ok, bad := 0, 0
	report := func(name string, cond bool, detail string) {
		if cond {
			fmt.Fprintf(stdout, "   ✓ %s\n", name)
			ok++
			return
		}
		fmt.Fprintf(stdout, "   ✗ %s：%s\n", name, detail)
		bad++
	}

	// 甲 缺件负控：无 --rollback ⇒ rc=2。
	{
		var b bytes.Buffer
		rc := cpaccRunRollback("", cpaccOpts{}, &b)
		report("甲 缺 --rollback ⇒ 不给结论（rc=2）",
			rc == cpaccRCBlocked && strings.Contains(b.String(), "缺件"),
			fmt.Sprintf("期望 rc=2 且报「缺件」，得 rc=%d out=%q", rc, b.String()))
	}
	work := cpaccMkTemp("cpacc-rb-")
	if work != "" {
		defer os.RemoveAll(work)
	}
	state := cpaccMkTemp("cpacc-rbs-")
	if state != "" {
		defer os.RemoveAll(state)
	}
	if work == "" || state == "" {
		report("前置：临时目录造得出", false, "MkdirTemp 失败")
		fmt.Fprintf(stdout, "   自检：%d 格过 · %d 格不过（仓外临时目录 · 未碰真仓）\n", ok, bad)
		return cpaccSelfTestRC(bad)
	}
	file := filepath.Join(work, "artifact.bin")
	if err := os.WriteFile(file, []byte("payload\n"), 0o644); err != nil {
		report("前置：夹具件造得出", false, err.Error())
		fmt.Fprintf(stdout, "   自检：%d 格过 · %d 格不过（仓外临时目录 · 未碰真仓）\n", ok, bad)
		return cpaccSelfTestRC(bad)
	}
	// 乙 负控：记录值缺位 ⇒ rc=1。
	{
		var b bytes.Buffer
		rc := cpaccRunRollback(work, cpaccOpts{rollback: file, stateDir: state}, &b)
		report("乙 记录值缺位 ⇒ rc=1",
			rc == cpaccRCFail && strings.Contains(b.String(), "记录值缺位"),
			fmt.Sprintf("期望 rc=1 且报「记录值缺位」，得 rc=%d out=%q", rc, b.String()))
	}
	// 丙 正控：记录值一致 ⇒ rc=0。
	{
		now := cpaccSha256File(file)
		if now == "" {
			report("前置：shasum 在盘", false, "shasum 取不到")
			fmt.Fprintf(stdout, "   自检：%d 格过 · %d 格不过（仓外临时目录 · 未碰真仓）\n", ok, bad)
			return cpaccSelfTestRC(bad)
		}
		recDir := filepath.Join(state, cpaccArchivePrefix+time.Now().Format(cpaccDateLayout))
		if err := os.MkdirAll(recDir, 0o755); err != nil {
			report("前置：记录值目录造得出", false, err.Error())
			fmt.Fprintf(stdout, "   自检：%d 格过 · %d 格不过（仓外临时目录 · 未碰真仓）\n", ok, bad)
			return cpaccSelfTestRC(bad)
		}
		rec := filepath.Join(recDir, devEditSafeName(file)+".sha256")
		if err := os.WriteFile(rec, []byte(now+"  "+file+"\n"), 0o644); err != nil {
			report("前置：记录值件造得出", false, err.Error())
			fmt.Fprintf(stdout, "   自检：%d 格过 · %d 格不过（仓外临时目录 · 未碰真仓）\n", ok, bad)
			return cpaccSelfTestRC(bad)
		}
		var b bytes.Buffer
		rc := cpaccRunRollback(work, cpaccOpts{rollback: file, stateDir: state}, &b)
		report("丙 正控 现值==记录值 ⇒ rc=0",
			rc == cpaccRCOK, fmt.Sprintf("期望 rc=0，得 rc=%d out=%q", rc, b.String()))
	}
	fmt.Fprintf(stdout, "   自检：%d 格过 · %d 格不过（仓外临时目录 · 未碰真仓）\n", ok, bad)
	return cpaccSelfTestRC(bad)
}
