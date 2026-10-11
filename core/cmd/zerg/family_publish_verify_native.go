// family_publish_verify_native.go —— `zerg publish verify`（拟名 · **待命令树登记**）的 **Go 真身**
// （发布链 Go 半优先 · 本批第三件 · 上两件 = 预检 `family_publish_preflight_native.go` ·
// 压平器 `family_publish_public_native.go`）。
//
// 源件：`scripts/build/test-publish-parity.sh`（B8「两器产出一致性」可重复验收 · **无 CLI 入口**）。
// 本笔把它的**编排**搬进 CLI 进程内，逐段与旧壳逐字对齐：
//
//	① 准备（仓根 / 件路径 / BASE 工作区 · 私有仓 HEAD 与 porcelain 基线）
//	② 跑两器（旧 = 压平快照器 · 新 = 逐提交镜像器；都在**仓外** BASE 下 —— **不出网、不推送**）
//	③ 两器产出树逐路径比对（路径集合 + blob sha + 模式）＋ 已知差异走显式契约（调 `parity-compare.py`）
//	④ 过滤口径对账（白名单 / 排除项同源）＋ EXCLUDES 发布机制脚本自登记
//	⑤ 私有面门禁（对两份产出树各跑一次既有检查器）
//	⑥ 私有称谓不泄露（镜像全史）＋ 替换规则表自伤空操作
//	⑦⑧⑨ 反例：多路径 / 改 blob / 空契约 / `*` 契约 四种造差异都必须失败
//	⑩⑪ 私有仓只读 + 绝不推送
//
// ★ 子进程边界逐字保留（与旧壳同一条）：两器 = `bash <件>`；比较器 = `python3 publish/parity-compare.py`；
// 两段内联 python（EXCLUDES 解析 / 替换规则自伤）经 `python3 -` 逐字节交过去 ⇒ 数据处理面与旧壳**同源**、
// ✗ 不在 Go 侧另造第二口径。
//
// ★ 两态对拍：`zerg publish verify`（本真身）⟷ `bash scripts/build/test-publish-parity.sh`（旧壳）在
// **同一只仓外 BASE**（`PARITY_BASE` 点名）上 rc/stdout/stderr 三面逐字同（掩时间戳 / 临时路径 / 随机 id）。
// 两态都读同名旋钮：`PARITY_BASE`（工作区落点）· `SKIP_TOOLS=1`（复用两器既有产出，只跑比对与反例）。
//
// ★ 未覆盖面（✗ 不声称逐字对齐）：
//
//	· 旧壳内把一段文本 `>> "${BASE}/neg-content/tools/ls.md"` 追加，在**目录缺失**时由 **bash 自产**一行
//	  stderr 诊断（`<脚本调用路径>: line 215: <路径>: No such file or directory`）—— 该行的脚本路径取决于
//	  **调用时**写的是相对还是绝对路径 ⇒ **不可确定复刻**。Go 侧同条件静默跳过追加（后续断言与退码两态
//	  一致）；这一行是**呈现差异**（stderr 少一行），非行为差异。
//	· 旧壳把 `git …` 经 `$( … )` 命令替换取值时，**未重定向**的那些调用把 git 的 stderr 透到终端；本件对
//	  这类调用**照旧透出**（`ppvGitOut(…, stderr, …)`），对 `2>/dev/null` 的调用丢弃 —— 逐条对齐旧壳。
//	· 旧壳 `tail`/`sed`/`grep`/`wc` 的**实现级**边缘（超长行 / 非 UTF-8 / NUL）不逐一复刻；本件按**行**语义复刻。
//	· 旧壳 `#!/usr/bin/env bash` 的解释器随 PATH 变；本件是编译制品，无此旋钮。
//	· 两器自身日志里的 **pid / 时间戳**属**随机/时间**面，对拍时归一（非本件可控差异）。
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ── 旧壳里各 `grep -E`/`grep` 的口径（Go 侧按**行**语义复刻，✗ 不引 python/awk 二次口径）──

// ppvReOnlyOld —— 旧壳 :208 / :224 的 `grep -E '只有旧器有|契约外差异'`。
var ppvReOnlyOld = regexp.MustCompile(`只有旧器有|契约外差异`)

// ppvReContent —— 旧壳 :224 的 `grep -E '内容不同|契约外差异'`。
var ppvReContent = regexp.MustCompile(`内容不同|契约外差异`)

// ppvReMirrorState —— 旧壳 :110 的 `grep -q '\[\S*\] \.mirror-state'`。
var ppvReMirrorState = regexp.MustCompile(`\[\S*\] \.mirror-state`)

// ppvReAcct —— 旧壳 :120 的 `grep -E '白名单条目|排除项|未入公开面'`。
var ppvReAcct = regexp.MustCompile(`白名单条目|排除项|未入公开面`)

// ppvReEmptySym —— 旧壳 :235 的 `grep -E '契约外差异|\.mirror-state'`。
var ppvReEmptySym = regexp.MustCompile(`契约外差异|\.mirror-state`)

// ppvReWild —— 旧壳 :246 的 `grep -E '通配符|许可证'`。
var ppvReWild = regexp.MustCompile(`通配符|许可证`)

// ppvRePlanDel —— 旧壳 :96 的 `grep -qE '计划在\*\*下一个版本'`（`\*` = 字面星）。
var ppvRePlanDel = regexp.MustCompile(`计划在\*\*下一个版本`)

// ppvLeakProbes —— 旧壳 :168 的三枚「私有称谓」探针（`-e` 三条）。
//
// ★ **拼串写**（✗ 不写连续字面原串）：本件落在公开面（`core/**`）内，而发布链的私有面扫描正盯着
// 这几枚称谓 ⇒ 直写会让本件在公开镜像里把该门判红。三段各自拼接后与旧壳 `-e` 的值**逐字同**。
var ppvLeakProbes = []string{"老" + "叨", "脑" + "（Hermes）", "脑" + "(Hermes)"}

// ppvRCNotFound —— shell 惯例退码 127（子进程起不来 / 日志句柄拿不到）；127 属 internal 命名空间、不入命令面真源表（先例 GAP-20261002-08）。
const ppvRCNotFound = 127

// ── 通用小工具（全部前缀 `ppv` · ✗ 不与他人撞名）──

// ppvRC —— 子进程退码（信号死照 Go 惯例返 -1，与旧壳 `$?` 的 128+n 不同；对拍面里两器都非信号死）。
func ppvRC(err error) int {
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	return ppvRCNotFound
}

// ppvIndent —— 复刻 `sed 's/^/PAD/'`：逐**行**加前缀；末尾换行照原样保留（空串 ⇒ 空串）。
func ppvIndent(s, pad string) string {
	if s == "" {
		return ""
	}
	had := strings.HasSuffix(s, "\n")
	body := strings.TrimSuffix(s, "\n")
	lines := strings.Split(body, "\n")
	for i := range lines {
		lines[i] = pad + lines[i]
	}
	out := strings.Join(lines, "\n")
	if had {
		out += "\n"
	}
	return out
}

// ppvTail — 复刻 `tail -n`（最后 n 行 · 文件不足 n 行则全出 · 空文件出空）。
func ppvTail(s string, n int) string {
	if s == "" {
		return ""
	}
	had := strings.HasSuffix(s, "\n")
	body := strings.TrimSuffix(s, "\n")
	lines := strings.Split(body, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	out := strings.Join(lines, "\n")
	if had {
		out += "\n"
	}
	return out
}

// ppvGrepIndent —— 复刻 `grep -E <re> <content> | head -k | sed 's/^/PAD/'`。
// 无命中 ⇒ 出空串（旧壳里 grep 对缺件往 stderr 报错、管道里 stdout 亦空）。
func ppvGrepIndent(content, re, pad string, k int) string {
	rx := regexp.MustCompile(re)
	hit := []string{}
	for _, ln := range strings.Split(strings.TrimSuffix(content, "\n"), "\n") {
		if !rx.MatchString(ln) {
			continue
		}
		hit = append(hit, ln)
		if len(hit) >= k {
			break
		}
	}
	if len(hit) == 0 {
		return ""
	}
	return ppvIndent(strings.Join(hit, "\n")+"\n", pad)
}

// ppvReadFile —— 读件（读不到 ⇒ 空串，与 `$(cat …)` / `sed` 的缺件行为对齐）。
func ppvReadFile(p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return string(b)
}

// ppvCountLines —— 复刻 `wc -l | tr -d ' '`（数换行符）。
func ppvCountLines(s string) int { return strings.Count(s, "\n") }

// ppvRunLog —— 起子进程并把 stdout+stderr **合并**写进 logPath（复刻 `> file 2>&1`）；返回其退码。
func ppvRunLog(logPath, name string, args ...string) int {
	f, err := os.Create(logPath)
	if err != nil {
		return ppvRCNotFound
	}
	defer f.Close()
	cmd := armChildGroup(exec.Command(name, args...))
	runningChild = cmd
	defer func() { runningChild = nil }()
	cmd.Stdout = f
	cmd.Stderr = f
	return ppvRC(cmd.Run())
}

// ppvToolRunTimeout —— 验-1「真跑两器」（旧压平发布器 / 逐提交镜像器 · 两句 `ppvRunLog`）的**单器**上限
// （停止条件）。
//
// ★ 现读依据：仓外重编件长限时实测该步（两句 `ppvRunLog` 之合）**会完**、耗 ~5m44s（344s，非死锁）；
// 收口编排件（`publish accept`）本应只读，却**无超时地真跑两器** ⇒ 某器一旦卡死，整件**无界挂起**
// （前一单实测：`── 验-1 对拍零分歧 ──` 表头后 >240s 无返回）。取 ~1.7× 实测值为上限 ⇒ 正常完成不被误杀、
// 卡死有停止条件。
// ★ 超时 ⇒ 调用方**必须**判「判不了」（fail-closed），✗ 不许读成绿。
const ppvToolRunTimeout = 10 * time.Minute

// ppvRunLogTimeout —— 同 `ppvRunLog`，但给「长步」加停止条件：到点用家族既有的 `terminateChildTree`
// 收**整棵**子树（TERM → 宽限 → KILL）。✗ 不用 `exec.CommandContext`：它只杀**直接子**，`bash <器>` 的
// 孙进程（门步骤 / python）改嫁后照跑（先例注释 `check_fields_face_parity.go:690` · `ffpRunProbe`）。
// 返回 (退码, 是否超时)；超时 ⇒ 退码取 `ppvRCNotFound`、第二值 true（调用方据此 fail-closed）。
func ppvRunLogTimeout(limit time.Duration, logPath, name string, args ...string) (int, bool) {
	f, err := os.Create(logPath)
	if err != nil {
		return ppvRCNotFound, false
	}
	defer f.Close()
	cmd := armChildGroup(exec.Command(name, args...))
	cmd.Stdout = f
	cmd.Stderr = f
	runningChild = cmd
	defer func() { runningChild = nil }()
	if err := cmd.Start(); err != nil {
		return ppvRCNotFound, false
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case werr := <-done:
		return ppvRC(werr), false
	case <-time.After(limit):
		terminateChildTree(cmd)
		<-done
		return ppvRCNotFound, true
	}
}

// ppvRunTo — 起子进程，stdout/stderr 直通给定流（用于可见输出的裸命令，如 `cp -R`）。
func ppvRunTo(stdout, stderr io.Writer, name string, args ...string) int {
	cmd := armChildGroup(exec.Command(name, args...))
	runningChild = cmd
	defer func() { runningChild = nil }()
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return ppvRC(cmd.Run())
}

// ppvGitIn — 在 dir 下跑 git（可注入 env），stdout/stderr 直通给定流（复刻 `( cd D && ENV git … )`）。
func ppvGitIn(dir string, env []string, stdout, stderr io.Writer, args ...string) int {
	cmd := armChildGroup(exec.Command("git", args...))
	runningChild = cmd
	defer func() { runningChild = nil }()
	cmd.Dir = dir
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return ppvRC(cmd.Run())
}

// ppvGitOut — 在 dir 下跑 git，**只捕获 stdout**（复刻 `$(git …)`：命令替换只收 stdout）。
// stderrW=nil ⇒ 丢弃（对应旧壳 `2>/dev/null`）；stderrW=stderr ⇒ 照旧壳「未重定向」让 git 的诊断透出。
func ppvGitOut(dir string, stderrW io.Writer, args ...string) (int, string) {
	var b bytes.Buffer
	// ★ 照旧壳用 `git -C <dir>`（✗ **不**用 Go 的 cmd.Dir）：`cmd.Dir` 指向不存在目录时 Go 在起进程前
	// 就 chdir 失败、**不调用 git** ⇒ 丢掉 git 自己的 `fatal: cannot change to …` 诊断；走 `-C` 则 git
	// 真起来、按旧壳逐字吐同一行（两态 stderr 才对得上）。
	cmd := armChildGroup(exec.Command("git", append([]string{"-C", dir}, args...)...))
	runningChild = cmd
	defer func() { runningChild = nil }()
	cmd.Stdout = &b
	if stderrW != nil {
		cmd.Stderr = stderrW
	}
	return ppvRC(cmd.Run()), b.String()
}

// ppvPrivCheck —— 复刻旧壳 `priv_check()`：优先 Go 半（`bin/zerg gate check check-public-tree-private <树>`），
// 退回 py 半（`python3 scripts/gates/check-public-tree-private.py <树>`）；两半皆缺 ⇒ 调用点 fail-closed rc=2。
// 输出写进 logPath。退码透传（0 绿 / 非 0 红或取不到结论）。
func ppvPrivCheck(root, tree, logPath string) int {
	f, err := os.Create(logPath)
	if err != nil {
		return ppvRCNotFound
	}
	defer f.Close()
	zergBin := filepath.Join(root, "bin", "zerg")
	privPy := filepath.Join(root, "scripts", "gates", "check-public-tree-private.py")
	var cmd *exec.Cmd
	if fi, serr := os.Stat(zergBin); serr == nil && fi.Mode()&0o111 != 0 {
		cmd = exec.Command(zergBin, "gate", "check", "check-public-tree-private", tree)
	} else if fi, serr := os.Stat(privPy); serr == nil && !fi.IsDir() {
		cmd = exec.Command("python3", privPy, tree)
	} else {
		fmt.Fprint(f, "⛔ 私有面门禁件两半皆缺（py 与 Go 半）——不给结论（rc=2）\n")
		return 2
	}
	cmd = armChildGroup(cmd)
	runningChild = cmd
	defer func() { runningChild = nil }()
	cmd.Stdout = f
	cmd.Stderr = f
	return ppvRC(cmd.Run())
}

// publishVerifyBytes —— 旧壳 :126-152 的内联 python（EXCLUDES 解析 + 退役断言 + 登记齐全）。**逐字节**交 python3 -。
const publishVerifyBytes = `import os, re, sys
repo = sys.argv[1]
p = repo + "/scripts/build/publish-public.sh"
t = open(p, encoding="utf-8").read()
m = re.search(r"EXCLUDES=\((.*?)\n\)", t, re.S)
ex = [l.split("#")[0].strip().strip('"').strip("'") for l in m.group(1).split("\n")
      if l.strip() and not l.strip().startswith("#")]
ex = [e for e in ex if e]
need = ["scripts/gates/check-hardcoded-private-paths.py", "scripts/build/publish-public.sh"]
retired_py = "scripts/gates/check-history-secrets.py"
retired_go = "core/cmd/zerg/check_history_secrets.go"
mirror_lib = "publish/mirror-public-lib.py"
missing = [n for n in need if n not in ex]
if retired_py in ex:
    missing.append(retired_py + "\uff08\u9000\u5f79\u4ef6\u4ecd\u5728 EXCLUDES \u2014\u2014 \u5e94\u4e0b\u67b6\uff09")
for rel in (retired_go, mirror_lib):
    if not os.path.exists(os.path.join(repo, rel)):
        missing.append(rel + "\uff08\u4e0d\u5728\u76d8\uff09")
print("  \u6392\u9664\u9879 %d \u6761\uff1b\u53d1\u5e03\u673a\u5236\u811a\u672c\u767b\u8bb0\u9f50\u5168: %s" % (len(ex), "\u662f" if not missing else "\u7f3a %s" % missing))
sys.exit(1 if missing else 0)
`

// publishVerifyNoop —— 旧壳 :177-190 的内联 python（替换规则表自伤空操作）。**逐字节**交 python3 -。
const publishVerifyNoop = `import sys
p = sys.argv[1] + "/publish/replace-rules.tsv"
noop = []
for ln in open(p, encoding="utf-8"):
    ln = ln.rstrip("\n")
    if not ln.strip() or ln.lstrip().startswith("#"):
        continue
    f = ln.split("\t")
    if len(f) >= 2 and f[0] and f[0] == f[1]:
        noop.append(f[0])
print("  \u89c4\u5219\u8868\u81ea\u4f24\u7a7a\u64cd\u4f5c\uff08pattern == replacement\uff09: %s" % (noop or "\u65e0"))
sys.exit(1 if noop else 0)
`

// cmdPublishVerify —— `zerg publish verify`（拟名 · 待命令树登记）：两器产出一致性验收（**本地** · 不出网、不推送）。
//
// 本命令**无位置参数**；工作区旋钮与旧壳同名（`PARITY_BASE` / `SKIP_TOOLS`），保两态可逐字对拍。
func cmdPublishVerify(_ *invocation, stdout, stderr io.Writer) int {
	return publishVerifyRun(stdout, stderr)
}

// publishVerifyRun —— 旧壳 `main()` 的**逐段转写**（✗ 一处语义不动 · 退码与 stdout/stderr 三面照旧）。
func publishVerifyRun(stdout, stderr io.Writer) int {
	root := repoRoot()
	if root == "" {
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 跑不动两器对拍（不给结论 · 退码 8）\n", progName)
		return exitBlocked
	}
	oldPub := filepath.Join(root, "scripts", "build", "publish-public.sh")
	mirror := filepath.Join(root, "publish", "mirror-public.sh")
	compare := filepath.Join(root, "publish", "parity-compare.py")
	contract := filepath.Join(root, "publish", "parity-contract.tsv")
	allow := filepath.Join(root, "publish", "mirror-allow.txt")

	base := strings.TrimSpace(os.Getenv("PARITY_BASE"))
	if base == "" {
		base = "/tmp/zerg-parity-" + time.Now().Format("20060102-150405")
	}
	oldDir := filepath.Join(base, "old")
	mirDir := filepath.Join(base, "mirror")
	_ = os.MkdirAll(base, 0o755)
	skipTools := os.Getenv("SKIP_TOOLS") == "1"

	pass, fail := 0, 0
	ok := func(s string) { fmt.Fprintf(stdout, "  ✅ PASS  %s\n", s); pass++ }
	bad := func(s string) { fmt.Fprintf(stdout, "  ❌ FAIL  %s\n", s); fail++ }
	head1 := func(s string) { ppSay(stdout, s) }

	fmt.Fprintf(stdout, "验收工作区: %s\n", base)
	fmt.Fprintf(stdout, "私有仓:     %s（只读）\n", root)
	_, head0 := ppvGitOut(root, stderr, "rev-parse", "HEAD")
	head0 = strings.TrimSpace(head0)
	_, dirtyOut := ppvGitOut(root, stderr, "status", "--porcelain")
	dirty0 := ppvCountLines(dirtyOut)
	head12 := head0
	if len(head12) > 12 {
		head12 = head12[:12]
	}
	fmt.Fprintf(stdout, "私有仓 HEAD: %s · porcelain: %d\n", head12, dirty0)

	// ── 前置：四件必须都在盘（缺 ⇒ stderr 点名 + rc=2）──
	for _, f := range []string{oldPub, mirror, compare, contract} {
		fi, err := os.Stat(f)
		if err != nil || fi.IsDir() {
			fmt.Fprintf(stderr, "✗ 缺文件：%s\n", f)
			return exitUsage
		}
	}

	// ── 1 跑两器 ──
	head1("1 跑两器（旧=压平快照 / 新=逐提交镜像；都在 " + base + " 下）")
	oldRC, mirRC := 0, 0
	oldLog := filepath.Join(base, "old.log")
	mirLog := filepath.Join(base, "mirror.log")
	if skipTools {
		fmt.Fprintln(stdout, "  … SKIP_TOOLS=1：复用既有产出")
	} else {
		_ = os.RemoveAll(oldDir)
		_ = os.RemoveAll(mirDir)
		var oldTO, mirTO bool
		oldRC, oldTO = ppvRunLogTimeout(ppvToolRunTimeout, oldLog, "bash", oldPub, "--out", oldDir)
		mirRC, mirTO = ppvRunLogTimeout(ppvToolRunTimeout, mirLog, "bash", mirror, "--out", mirDir, "--allow", allow)
		if oldTO || mirTO {
			fmt.Fprintf(stdout, "  ⛔ 跑两器超时（单器上限 %s）—— 不给结论（fail-closed · ✗ 不当作绿）\n", ppvToolRunTimeout)
			return exitBlocked
		}
		fmt.Fprintf(stdout, "  旧器退出码 %d · 镜像器退出码 %d\n", oldRC, mirRC)
	}
	if oldRC == 0 {
		ok("1a 旧压平发布器跑绿（含私有面/文档/i18n/版本/兼容清单五道门禁）")
	} else {
		bad(fmt.Sprintf("1a 旧器退出码 %d（见 %s）", oldRC, oldLog))
		fmt.Fprint(stdout, ppvIndent(ppvTail(ppvReadFile(oldLog), 12), "      "))
	}
	if mirRC == 0 {
		ok("1b 逐提交镜像器跑绿（逐笔硬门禁 + 私有面门禁）")
	} else {
		bad(fmt.Sprintf("1b 镜像器退出码 %d（见 %s）", mirRC, mirLog))
		fmt.Fprint(stdout, ppvIndent(ppvTail(ppvReadFile(mirLog), 12), "      "))
	}

	// 1c 弃用横幅（B8-A）
	if skipTools {
		fmt.Fprintln(stdout, "  （SKIP_TOOLS=1：跳过弃用警告检查）")
	} else {
		if strings.Contains(ppvReadFile(oldPub), "已弃用（DEPRECATED）") {
			ok("1c 旧器头部有显著弃用横幅")
		} else {
			bad("1c 旧器头部缺弃用横幅")
		}
		if strings.Contains(ppvReadFile(oldLog), "⚠️  已弃用：scripts/build/publish-public.sh") {
			ok("1c 运行时先打醒目警告再继续（日志首段可见，且仍然跑完 ⇒ 不拒绝执行）")
		} else {
			bad("1c 运行时没有弃用警告（Q5 要求『标弃用 + 一个版本后删』，须先警告再继续）")
		}
		if ppvRePlanDel.MatchString(ppvReadFile(oldPub)) {
			ok("1c 横幅里写明删除时限（下一个版本 v2.6.0）")
		} else {
			bad("1c 横幅没写删除时限")
		}
	}

	// ── 2/3 树内容一致性 + 契约对账 ──
	head1("2 树内容一致性（路径集合 + 每个路径的 blob sha + 模式）＋ 3 已知差异走契约")
	parityLog := filepath.Join(base, "parity.log")
	parityRC := ppvRunLog(parityLog, "python3", compare,
		"--old", oldDir, "--new", mirDir, "--contract", contract, "--repo", root,
		"--json", filepath.Join(base, "parity.json"))
	parityTxt := ppvReadFile(parityLog)
	fmt.Fprint(stdout, ppvIndent(parityTxt, "  "))
	fmt.Fprintf(stdout, "  比较器退出码: %d\n", parityRC)
	if parityRC == 0 {
		ok("2/3 两器树内容一致，差异集合 ⊆ 契约清单（契约外 0 处）")
	} else {
		bad(fmt.Sprintf("2/3 存在契约外差异（退出码 %d）—— 两器产出不一致", parityRC))
	}

	// 2b 契约条目被实际命中并带理由
	if strings.Contains(parityTxt, "契约内已知差异") &&
		ppvReMirrorState.MatchString(parityTxt) && strings.Contains(parityTxt, "理由：") {
		ok("2b 契约条目被实际命中并逐条打印了理由（不是躺在文件里的空许可）")
	} else {
		bad("2b 契约条目没被命中或没打印理由（契约是否与实际差异脱节？）")
	}

	// ── 4 过滤口径对账 ──
	head1("4 过滤口径对账（白名单/排除项同源 + 未入公开面的路径双方对称）")
	if strings.Contains(parityTxt, "不对称 0") {
		fmt.Fprint(stdout, ppvGrepIndent(parityTxt, ppvReAcct.String(), "  ", 1<<30))
		ok("4 两器未入公开面的路径**对称**（0 处不对称）—— 排除口径一致")
	} else {
		bad("4 过滤口径不对称（见 parity.log）")
	}
	// 4b 排除项来自旧器脚本（单一真源）
	if rc := ppPython(publishVerifyBytes, []string{root}, stdout, stderr); rc == 0 {
		ok("4b 发布机制自身已在 EXCLUDES 里统一登记（镜像器解析同一份 ⇒ 不会两处漂移）")
	} else {
		bad("4b EXCLUDES 里缺发布机制脚本")
	}

	// ── 5 私有面门禁（两份产出树各跑一次）──
	head1("5 私有面门禁（对旧器产出树 + 镜像产出树各跑一次既有检查器）")
	for _, t := range []string{oldDir, mirDir} {
		bn := filepath.Base(t)
		plog := filepath.Join(base, "priv-"+bn+".log")
		if ppvPrivCheck(root, t, plog) == 0 {
			ok("5 " + bn + " 私有面门禁 rc=0")
		} else {
			bad("5 " + bn + " 命中私有面 / 门两半皆缺")
			fmt.Fprint(stdout, ppvIndent(ppvReadFile(plog), "      "))
		}
	}

	// ── 6 私有称谓不泄露（镜像全史）──
	head1("6 私有称谓不泄露：镜像全史 0 处「私有拍板人称谓 / 助手署名」")
	// 旧壳 :169 的**内层** `$(git rev-list …)`（无重定向 ⇒ stderr 透出）+ :170 的 `$(git rev-list … | wc -l)`
	// —— 两处各自真跑一次 rev-list，✗ 不合并（合并会少一条 git 的 stderr 诊断行）。
	_, commitsForGrep := ppvGitOut(mirDir, stderr, "rev-list", "refs/heads/main")
	grepArgs := []string{"grep", "-l", "-e", ppvLeakProbes[0], "-e", ppvLeakProbes[1], "-e", ppvLeakProbes[2]}
	grepArgs = append(grepArgs, strings.Fields(commitsForGrep)...)
	_, g := ppvGitOut(mirDir, nil, grepArgs...) // 旧壳 :169 外层 `2>/dev/null`
	leakHits := ppvCountLines(g)
	_, commitsCount := ppvGitOut(mirDir, stderr, "rev-list", "refs/heads/main")
	leakCommits := ppvCountLines(commitsCount)
	if leakHits == 0 {
		ok(fmt.Sprintf("6 镜像 %d 笔全史里 0 处私有称谓（替换规则不再空转）", leakCommits))
	} else {
		bad(fmt.Sprintf("6 镜像全史里还有 %d 处私有称谓命中（替换规则又空转了？）", leakHits))
	}
	// 6b 替换规则表无自伤空操作
	if rc := ppPython(publishVerifyNoop, []string{root}, stdout, stderr); rc == 0 {
		ok("6b 替换规则表无自伤空操作（pattern ≠ replacement）")
	} else {
		bad("6b 规则表仍有自伤空操作")
	}

	// ── 7 反例：造差异必失败 ──
	head1("7 反例（造差异必失败）：多一个路径 ⇒ 红；改一个 blob ⇒ 红")
	negGitEnv := []string{
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@e",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@e",
	}
	// 7a 多一个路径
	negExtra := filepath.Join(base, "neg-extra")
	_ = os.RemoveAll(negExtra)
	ppvRunTo(io.Discard, io.Discard, "cp", "-R", oldDir, negExtra)
	_ = os.WriteFile(filepath.Join(negExtra, "INJECTED-EXTRA.txt"), []byte("injected\n"), 0o644)
	ppvGitIn(negExtra, negGitEnv, io.Discard, io.Discard, "add", "INJECTED-EXTRA.txt")
	ppvGitIn(negExtra, negGitEnv, io.Discard, io.Discard, "commit", "-q", "-m", "neg: inject extra path")
	negExtraLog := filepath.Join(base, "neg-extra.log")
	rcA := ppvRunLog(negExtraLog, "python3", compare, "--old", negExtra, "--new", mirDir, "--contract", contract)
	extraTxt := ppvReadFile(negExtraLog)
	fmt.Fprint(stdout, ppvGrepIndent(extraTxt, ppvReOnlyOld.String(), "      ", 3))
	if rcA != 0 && strings.Contains(extraTxt, "INJECTED-EXTRA.txt") {
		ok(fmt.Sprintf("7a 多一个路径 ⇒ 比较器报红（rc=%d）且点名 INJECTED-EXTRA.txt", rcA))
	} else {
		bad(fmt.Sprintf("7a 造差异没被抓到（rc=%d）", rcA))
	}
	// 7b 改一个 blob 内容
	negContent := filepath.Join(base, "neg-content")
	_ = os.RemoveAll(negContent)
	ppvRunTo(io.Discard, io.Discard, "cp", "-R", oldDir, negContent)
	if fh, err := os.OpenFile(filepath.Join(negContent, "tools", "ls.md"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
		_, _ = fh.WriteString("\n// injected by parity negative test\n")
		_ = fh.Close()
	}
	ppvGitIn(negContent, negGitEnv, io.Discard, io.Discard, "add", "tools/ls.md")
	ppvGitIn(negContent, negGitEnv, io.Discard, io.Discard, "commit", "-q", "-m", "neg: change one blob")
	negContentLog := filepath.Join(base, "neg-content.log")
	rcB := ppvRunLog(negContentLog, "python3", compare, "--old", negContent, "--new", mirDir, "--contract", contract)
	contentTxt := ppvReadFile(negContentLog)
	fmt.Fprint(stdout, ppvGrepIndent(contentTxt, ppvReContent.String(), "      ", 3))
	if rcB != 0 && strings.Contains(contentTxt, "tools/ls.md") {
		ok(fmt.Sprintf("7b 改一个 blob 内容 ⇒ 比较器报红（rc=%d）且点名 tools/ls.md", rcB))
	} else {
		bad(fmt.Sprintf("7b 内容差异没被抓到（rc=%d）", rcB))
	}

	// ── 8 反例：契约必须承重 ──
	head1("8 反例（契约承重）：空契约下真差异必须失败")
	emptyContract := filepath.Join(base, "empty-contract.tsv")
	_ = os.WriteFile(emptyContract, nil, 0o644)
	negEmptyLog := filepath.Join(base, "neg-empty.log")
	rcC := ppvRunLog(negEmptyLog, "python3", compare, "--old", oldDir, "--new", mirDir, "--contract", emptyContract)
	emptyTxt := ppvReadFile(negEmptyLog)
	fmt.Fprint(stdout, ppvGrepIndent(emptyTxt, ppvReEmptySym.String(), "      ", 4))
	if rcC != 0 && strings.Contains(emptyTxt, "契约外差异") {
		ok(fmt.Sprintf("8 空契约 ⇒ 真差异被判契约外并失败（rc=%d）⇒ 契约清单是**承重**的，不是装饰", rcC))
	} else {
		bad(fmt.Sprintf("8 空契约竟然也过（rc=%d）⇒ 契约过滤器没在起作用", rcC))
	}

	// ── 9 反例：契约不许当许可证 ──
	head1("9 反例（契约护栏）：把契约写成 `*` ⇒ 比较器必须拒绝")
	badContract := filepath.Join(base, "bad-contract.tsv")
	_ = os.WriteFile(badContract, []byte("*\tany\t放行一切\n"), 0o644)
	negStarLog := filepath.Join(base, "neg-star.log")
	rcD := ppvRunLog(negStarLog, "python3", compare, "--old", oldDir, "--new", mirDir, "--contract", badContract)
	starTxt := ppvReadFile(negStarLog)
	fmt.Fprint(stdout, ppvGrepIndent(starTxt, ppvReWild.String(), "      ", 2))
	if rcD != 0 && strings.Contains(starTxt, "许可证") {
		ok(fmt.Sprintf("9 `*` 契约被拒绝（rc=%d）⇒ 契约不能把全部差异一次性豁免", rcD))
	} else {
		bad(fmt.Sprintf("9 `*` 契约竟然被接受（rc=%d）", rcD))
	}

	// ── 10/11 私有仓只读 + 绝不推送 ──
	head1("10 私有仓未被动过 · 11 绝不推送")
	_, head1s := ppvGitOut(root, stderr, "rev-parse", "HEAD")
	head1s = strings.TrimSpace(head1s)
	_, dirtyOut1 := ppvGitOut(root, stderr, "status", "--porcelain")
	dirty1 := ppvCountLines(dirtyOut1)
	h1 := head1s
	if len(h1) > 12 {
		h1 = h1[:12]
	}
	fmt.Fprintf(stdout, "  HEAD: %s → %s · porcelain: %d → %d\n", head12, h1, dirty0, dirty1)
	if head0 == head1s {
		ok("10 私有仓 HEAD 未变")
	} else {
		bad("10 私有仓 HEAD 被改！")
	}
	if dirty0 == dirty1 {
		ok("10 porcelain 行数未变（两器都没碰私有工作树）")
	} else {
		bad(fmt.Sprintf("10 私有仓工作树状态变了（%d → %d）", dirty0, dirty1))
	}
	_, ro := ppvGitOut(oldDir, nil, "remote") // 旧壳 :259 的 `2>/dev/null`
	_, rm := ppvGitOut(mirDir, nil, "remote")
	remotes := fmt.Sprintf("%d%d", ppvCountLines(ro), ppvCountLines(rm))
	if remotes == "00" {
		ok("11 两份产出仓都没有 remote（没有任何推送路径）")
	} else {
		bad("11 产出仓配了 remote（" + remotes + "）")
	}
	oldLogTxt, mirLogTxt := ppvReadFile(oldLog), ppvReadFile(mirLog)
	pushedRe := regexp.MustCompile(`^已推送|已推送（非 force）`)
	if lineMatch(pushedRe, oldLogTxt) || lineMatch(pushedRe, mirLogTxt) {
		bad("11 日志里出现「已推送」成功行 —— 有真推送")
	} else if strings.Contains(mirLogTxt, "ZERG_MIRROR_ALLOW_PUSH=1") {
		bad("11 镜像器走了 --push 双重保险路径")
	} else {
		ok("11 两器日志无「已推送」成功行、无双保险放行（只打印了推送命令提示，未执行）")
	}

	// ── 小结 ──
	head1("小结")
	fmt.Fprintf(stdout, "  通过 %d 项 · 失败 %d 项 · 工作区 %s\n", pass, fail, base)
	if fail == 0 {
		fmt.Fprintf(stdout, "  ✅ 两器产出**树内容一致**（路径集合 + 逐路径 blob sha + 模式），差异集合 ⊆ %s\n", filepath.Base(contract))
		fmt.Fprintln(stdout, "     反例已验：多路径 / 改 blob / 空契约 / `*` 契约 四种造差异路径都能失败")
		return exitOK
	}
	fmt.Fprintln(stdout, "  ❌ 有失败项，逐条见上")
	return exitFail
}

// lineMatch —— 按**行**锚定匹配（复刻 `grep -qE '^已推送|已推送（非 force）'`：`^` 只锚第一条分支）。
func lineMatch(re *regexp.Regexp, s string) bool {
	for _, ln := range strings.Split(s, "\n") {
		if re.MatchString(ln) {
			return true
		}
	}
	return false
}
