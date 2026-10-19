// family_publish_preflight_native.go —— `zerg publish preflight` 的 **Go 真身**（发布链 Go 半优先 · 本批第一件）。
//
// 病（原壳）：`cmdPublishPreflight`（family_publish_run.go）只 `bash scripts/build/publish-preflight.sh <产物目录>`
// —— 编排（九道闸的调用序 / 计账 / 出口三档）长在 shell 里，CLI 面看不见真身。
// 本笔把编排**搬进 CLI 进程内**：逐闸取「合并输出 + 退码」、按旧壳的 `tail -n | sed` 逐字渲染，
// 出口三档（FAIL=1 / BLOCKED=2 / PASS=0）与旧壳逐字对齐。
//
// ★ 判据真源未动：每道闸仍**按名调用同族既有 Go 判据体**（`gateCheckGoEntry[...]` · 经
//
//	`gateCheckRunInproc` 走与 `zerg gate check <名>` 同一条进程内入口：chdir 到仓根 + 注入
//	ZERG_REPO/ZERG_REPO_ROOT）。本件**不复制任何判据实现**、不改任何闸的退码。
//
// ★ 两态对拍口径：`zerg publish preflight <D>`（本真身）⟷ `bash scripts/build/publish-preflight.sh <D>`（旧壳）
//
//	在同一夹具上 **rc/stdout/stderr 三面逐字同**（掩时间/临时路径/随机 id 后）。
//
// ★ 一处照实记的偏离：旧壳的每道闸调用会经 `zerg gate check` 落一行 judge-run 台账；本真身直调
//
//	判据体 ⇒ **不落该行**（台账是旁路副作用，不进 stdout/stderr ⇒ 不破两态三面；要补须另笔接线）。
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// publishPreflightRatchetInitial —— 闸⑧b 判据① 现值面**存量债棘轮**初值（照旧壳 `R8_STALE_RATCHET=2`）。
const publishPreflightRatchetInitial = 2

// pfRCNotFound —— shell 惯例退码 127（按名调判据体时闸名不在册）；127 属 internal 命名空间、不入命令面真源表（先例 GAP-20261002-08）。
const pfRCNotFound = 127

// pfExitSpawnFail —— shell 惯例退码 127（外部子进程（git / 宿主二进制）起不来 · 非 ExitError）。
// 属**底层壳件自有码空间**（✗ 不在命令面真源退码表 ⇒ ✗ 写裸数字，门⑪ T3）；与 `pfRCNotFound` 同值不同义；先例同上 `dplRCSSHAbsent`/`prExitSpawnFail`。
const pfExitSpawnFail = 127

// pfLeakScanCommitCap —— 闸③ 逐笔扫描的**提交数上限**（本单新增的「扫描上限」·照本件既有「判不了 ⇒ 记 BLOCKED」口径）。
// 现读 `git rev-list --count main` = 1607 ⇒ 取 4096（约 2.5× 头寸）：当前树全扫 ⇒ 与改前逐字相同；
// 全史超限（未来膨胀 / 异常输入）⇒ **不静默通过**（扫不完 ≠ 扫干净），交调用方记 BLOCKED（fail-closed）。
// ★ 不用 `rev-list -n<cap>`：那会**静默截断**（老提交里的泄漏被漏掉 ⇒ 假阴）—— 照实取全表再比。
const pfLeakScanCommitCap = 4096

// pfLeakScanOverCapRC —— 闸③「扫不完 ⇒ 判不了」的信号码（**不给结论**档 · 与「查到泄漏」档 1 分家）。
const pfLeakScanOverCapRC = 2

var (
	pfVerRe    = regexp.MustCompile(`(?m)^const Version = "([^"]+)"`)
	pfVerAnyRe = regexp.MustCompile(`(?m)^const Version = "([^"]*)"`)
	pfDocsTOK  = "2.5."
)

// pfTail —— 复刻旧壳 `tail -n <n> <log> | sed 's/^/   /'`（取合并输出末 n 行、逐行加前缀）。
func pfTail(w io.Writer, s string, n int, prefix string) {
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

// pfHead —— 复刻旧壳 `head -n <n> <log> | sed 's/^/   /'`（取合并输出首 n 行）。
func pfHead(w io.Writer, s string, n int, prefix string) {
	if s == "" {
		return
	}
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > n {
		lines = lines[:n]
	}
	for _, ln := range lines {
		fmt.Fprintf(w, "%s%s\n", prefix, ln)
	}
}

// pfGitOut —— 仓内 git（带 `-c core.quotepath=false`，逐字照旧壳内联 python 的 `git()`）；
// 返回 rc 与 **stdout 去尾空白**（python `p.stdout.strip()` —— 不含 stderr）。
func pfGitOut(dir string, args ...string) (int, string) {
	a := append([]string{"-C", dir, "-c", "core.quotepath=false"}, args...)
	cmd := armChildGroup(exec.Command("git", a...))
	runningChild = cmd
	defer func() { runningChild = nil }()
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	err := cmd.Run()
	rc := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			rc = ee.ExitCode()
		} else {
			rc = pfExitSpawnFail
		}
	}
	return rc, strings.TrimSpace(out.String())
}

// pfCallGate —— 按名调用同族既有 Go 判据体（进程内 · 与 `zerg gate check <名>` 同一条入口）；
// 返回 rc 与 **stdout+stderr 合并**输出（旧壳把每闸 `> /tmp/pfN.log 2>&1` 合并后取尾）。
func pfCallGate(root, name string, argv []string) (int, string) {
	fn, ok := gateCheckGoEntry[name]
	if !ok {
		return pfRCNotFound, ""
	}
	var buf bytes.Buffer
	rc := gateCheckRunInproc(fn, root, &buf, &buf, argv)
	return rc, buf.String()
}

// pfRunIn —— 在 dir 里跑 bin（照旧壳 `(cd <dir> && ENV bin args)`）；返回 rc 与合并输出。
// dir 不在盘 ⇒ rc=1、无输出（旧壳会向 stderr 打一条 bash 的 `cd: …: No such file or directory`，
// 那是 shell 实现细节；本真身不复刻该行 —— 见件头「两态对拍」口径，夹具用两条目录都在场的树）。
func pfRunIn(dir, bin string, args []string, extraEnv map[string]string) (int, string) {
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return 1, ""
	}
	cmd := armChildGroup(exec.Command(bin, args...))
	runningChild = cmd
	defer func() { runningChild = nil }()
	cmd.Dir = dir
	env := os.Environ()
	keys := make([]string, 0, len(extraEnv))
	for k := range extraEnv {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		env = append(env, k+"="+extraEnv[k])
	}
	cmd.Env = env
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	rc := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			rc = ee.ExitCode()
		} else {
			rc = pfExitSpawnFail
		}
	}
	return rc, out.String()
}

// pfCountPassed —— 复刻旧壳 `grep -oE '[0-9]+ passed' <log> | awk '{s+=$1} END {print s}'`：
// 无命中 ⇒ 空串（awk 未初始化变量打印空）。
func pfCountPassed(log string) string {
	re := regexp.MustCompile(`[0-9]+ passed`)
	ms := re.FindAllString(log, -1)
	if len(ms) == 0 {
		return ""
	}
	sum := 0
	for _, m := range ms {
		f := strings.Fields(m)
		if len(f) > 0 {
			if v, err := strconv.Atoi(f[0]); err == nil {
				sum += v
			}
		}
	}
	return strconv.Itoa(sum)
}

func pfIsDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func pfMax64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// pfTS —— 复刻 python `time.strftime("%Y-%m-%d %H:%M", localtime(t)) if t else "—"`。
func pfTS(t int64) string {
	if t == 0 {
		return "—"
	}
	return time.Unix(t, 0).Format("2006-01-02 15:04")
}

// pfReadVersionAny —— 读件里 `const Version = "…"`（允许空；照旧壳 gate⑥ 的 sed）。
func pfReadVersionAny(file string) string {
	raw, err := os.ReadFile(file)
	if err != nil {
		return ""
	}
	if m := pfVerAnyRe.FindStringSubmatch(string(raw)); m != nil {
		return m[1]
	}
	return ""
}

// publishPreflightNative —— `zerg publish preflight <产物目录>` 的进程内真身（旧壳逐字对齐）。
func publishPreflightNative(inv *invocation, stdout, stderr io.Writer, D string) int {
	Z := repoRoot()
	if Z == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 跑不动预检（不给结论 · 退码 8）\n", progName)
		return exitBlocked
	}
	fail := 0
	blocked := 0
	r8Note := 0
	r8RatchetRaised := 0

	// ── 闸⓪ 发布面同步 ──
	fmt.Fprintln(stdout, "── 闸⓪ 发布面同步（配置级·秒级，不依赖产物目录）──")
	rc0, log0 := pfCallGate(Z, "check-publish-face-sync", nil)
	pfTail(stdout, log0, 3, "   ")
	if rc0 != 0 {
		fmt.Fprintf(stdout, "   ✗ 闸⓪ 不过（rc=%d）\n", rc0)
		fail = 1
	}

	// ── 闸① 占位符截断残留 ──
	fmt.Fprintln(stdout, "── 闸① 占位符截断残留（逐笔 · 优先 Go 半）──")
	rc1, log1 := pfCallGate(Z, "check-placeholder-residue", []string{D, "--all"})
	pfTail(stdout, log1, 3, "   ")
	if rc1 != 0 {
		fmt.Fprintf(stdout, "   ✗ 闸① 不过（rc=%d）\n", rc1)
		fail = 1
	}

	// ── 闸② 私有面 ──
	fmt.Fprintln(stdout, "── 闸② 私有面（优先 Go 半）──")
	rc2, log2 := pfCallGate(Z, "check-public-tree-private", []string{D})
	pfTail(stdout, log2, 1, "   ")
	if rc2 != 0 {
		fmt.Fprintf(stdout, "   ✗ 闸② 不过（rc=%d）\n", rc2)
		fail = 1
	}

	// ── 闸②b 写死的私有绝对路径（私有源树）──
	fmt.Fprintln(stdout, "── 闸②b 写死的私有绝对路径（私有源树 · 内容级）──")
	rc2b, log2b := pfCallGate(Z, "check-hardcoded-private-paths", []string{Z})
	pfTail(stdout, log2b, 3, "   ")
	if rc2b != 0 {
		fmt.Fprintf(stdout, "   ✗ 闸②b 不过（rc=%d）\n", rc2b)
		fail = 1
	}

	// ── 闸③ 旧 token / 绝对路径（逐笔）──
	fmt.Fprintln(stdout, "── 闸③ 旧 token / 绝对路径（逐笔）──")
	rc3, log3 := pfLeakScan(D)
	pfTail(stdout, log3, 2, "   ")
	if rc3 == pfLeakScanOverCapRC {
		fmt.Fprintf(stdout, "   ⚠ 闸③ 判不了（rc=%d · 全史超扫描上限）⇒ 记 BLOCKED（缺件 ≠ 绿 · 不给结论 · 但也不许推送）\n", rc3)
		blocked = 1
	} else if rc3 != 0 {
		fmt.Fprintf(stdout, "   ✗ 闸③ 不过（rc=%d）\n", rc3)
		fail = 1
	}

	// ── 闸④ 危险路径与令牌形态（末树）──
	fmt.Fprintln(stdout, "── 闸④ 危险路径与令牌形态（末树）──")
	rc4, log4 := pfCallGate(Z, "check-public-tree-hazards", []string{D, "HEAD"})
	pfTail(stdout, log4, 2, "   ")
	if rc4 != 0 {
		fmt.Fprintf(stdout, "   ✗ 闸④ 不过（rc=%d）\n", rc4)
		fail = 1
	}

	// ── 闸⑤ 产物可构建（core go build + UI 镜像形态测试）──
	fmt.Fprintln(stdout, "── 闸⑤ 产物可构建（core go build + UI 镜像形态测试）──")
	rc5a, log5a := pfRunIn(filepath.Join(D, "core"), "go",
		[]string{"build", "-buildvcs=false", "./..."},
		map[string]string{"GOFLAGS": "-mod=mod", "GOSUMDB": "off", "GOPROXY": "https://goproxy.cn,direct"})
	fmt.Fprintf(stdout, "   core go build rc=%d\n", rc5a)
	if rc5a != 0 {
		pfHead(stdout, log5a, 3, "     ")
		fail = 1
	}
	rc5b, log5b := pfRunIn(filepath.Join(D, "ui"), "cargo",
		[]string{"test", "-p", "zerg-ui", "--no-default-features"},
		map[string]string{"CARGO_TARGET_DIR": "/tmp/pf-target"})
	fmt.Fprintf(stdout, "   ui 镜像形态 rc=%d | %s passed\n", rc5b, pfCountPassed(log5b))
	if rc5b != 0 {
		fail = 1
	}

	// ── 闸⑥ 清单新鲜度 ──
	fmt.Fprintln(stdout, "── 闸⑥ 清单新鲜度（发布制品 manifest.json · 缺件 ⇒ 不给结论）──")
	ver := pfReadVersionAny(filepath.Join(D, "core", "internal", "version", "version.go"))
	mf := ""
	for _, c := range []string{
		filepath.Join(D, "manifest.json"),
		filepath.Join(Z, "dist", ver, "release", "manifest.json"),
		filepath.Join(Z, "dist", ver, "manifest.json"),
	} {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			mf = c
			break
		}
	}
	if mf == "" {
		fmt.Fprintf(stdout, "   ⚠ 缺件：没有可判的 manifest.json（找过 $D/manifest.json · dist/%s/release/manifest.json · dist/%s/manifest.json）\n", ver, ver)
		fmt.Fprintln(stdout, "   ⚠ 「没得判」不是「绿」⇒ 记 BLOCKED（不给结论 · 不计失败项 · 但也不许推送）")
		fmt.Fprintln(stdout, "   ⚠ 处置：先跑 scripts/build/pack-release.sh 出同版清单，或给本闸传制品目录（见 .md §待拍）")
		blocked = 1
	} else {
		fmt.Fprintf(stdout, "   目标（版本 %s）：%s\n", ver, mf)
		rc6, log6 := pfCallGate(Z, "check-manifest-freshness", []string{mf, "--repo", Z, "--strict"})
		pfTail(stdout, log6, 3, "   ")
		if rc6 != 0 {
			fmt.Fprintf(stdout, "   ✗ 闸⑥ 不过（rc=%d）\n", rc6)
			fail = 1
		}
	}

	// ── 闸⑧ 公开文档面随版（分两刀 · 五条判据 ⑧a–⑧e）──
	fmt.Fprintln(stdout, "── 闸⑧ 公开文档面随版（分两刀 · 五条判据 ⑧a–⑧e · 缺件仍拦）──")
	FACE := os.Getenv("ZERG_PUBFACE_ROOT")
	if FACE == "" {
		FACE = Z
	}
	if FACE != Z {
		fmt.Fprintf(stdout, "   ⚠ 判据树被 ZERG_PUBFACE_ROOT 改指：%s（自检/负控夹具专用旋钮 · 与 ZERG_PUBLIC_TREE / ZERG_DEVDOCS_ROOT 同类）\n", FACE)
	}

	// ⑧a 红线
	fmt.Fprintln(stdout, "   ⑧a 红线（门㉝ = `T-03` 现成件 · 与提交闸同一把判据 · 不复制实现 · 明细 /tmp/pf8a.log）")
	rc8a, log8a := pfCallGate(Z, "check-public-face-wording", []string{"--root", FACE})
	pfTail(stdout, log8a, 4, "      ")

	// ⑧b 随版三项（内联判据）
	fmt.Fprintln(stdout, "   ⑧b 随版三项（现值面 / 版本串 / 成对性 · 内联判据 · 无新脚本件 · 明细 /tmp/pf8b.log）")
	rc8b, log8b, r8Stale := pfDocsFreshness(FACE)
	pfTail(stdout, log8b, 14, "      ")
	if r8Stale < 0 {
		fmt.Fprintln(stdout, "   ⚠ 存量债棘轮：读不到现读件数（⑧b 本跑没打出机读行）⇒ 判不了 ⇒ 记 BLOCKED（缺件 ≠ 绿）")
		rc8b = 2
	} else if r8Stale > publishPreflightRatchetInitial {
		fmt.Fprintf(stdout, "   ✗ 存量债棘轮**上调**：现读 %d 件 > 初值 %d 件 ⇒ 即判红（只许降不许升 · 照 `baseline-ratchet.py` 形态）\n", r8Stale, publishPreflightRatchetInitial)
		r8RatchetRaised = 1
	} else {
		fmt.Fprintf(stdout, "   ✓ 存量债棘轮：现读 %d 件 ≤ 初值 %d 件（只许降不许升）\n", r8Stale, publishPreflightRatchetInitial)
	}

	// ⑧c 命令名（must-fail 负控 + 真跑）
	rc8cST, log8cST := pfCallGate(Z, "check-public-face-commands", []string{"--self-test"})
	pfTail(stdout, log8cST, 2, "      ")
	if rc8cST != 0 {
		fmt.Fprintf(stdout, "   ⛔ ⑧c 的 must-fail 负控（该件 --self-test）不过（rc=%d）⇒ 判据自身没牙 ⇒ 记 BLOCKED\n", rc8cST)
	}
	rc8c, log8c := pfCallGate(Z, "check-public-face-commands", []string{"--root", FACE, "--no-self-test"})
	pfTail(stdout, log8c, 6, "      ")
	fmt.Fprintln(stdout, "   ⑧c 命令名（判据 V10）── **已接线**（T-14 已落地 ⇒ 按名调用 check-public-face-commands.py）· 原申报留痕：`T-14` 未落地（其前置页 = `T-10` 未落）⇒ ★ 该申报**已作废**（T-14 已落地 · 见上行那次调用）")

	// ⑧d 成对闸
	ver8d := pfReadVersionAny(filepath.Join(FACE, "core", "internal", "version", "version.go"))
	base8d := ""
	if ver8d != "" {
		if rc, out := pfGitOut(FACE, "log", "-1", "--format=%H", "-S", `const Version = "`+ver8d+`"`, "--", "core/internal/version/version.go"); rc == 0 {
			base8d = strings.TrimSpace(out)
		}
	}
	baseDisp := base8d
	if baseDisp == "" {
		baseDisp = "<取不到>"
	}
	verDisp := ver8d
	if verDisp == "" {
		verDisp = "<读不出>"
	}
	fmt.Fprintf(stdout, "   ⑧d 成对闸（T-12）── 按名调用 check-public-face-pairing.py · 判据面 = 本版窗口「%s → 工作树」（V=%s）· 明细 /tmp/pf8d.log\n", baseDisp, verDisp)
	rc8d := 0
	rc8dST := 0
	var log8dST string
	var log8d string
	if base8d == "" {
		pfTail(stdout, fmt.Sprintf("   ⛔ ⑧d 取不到本版窗口基点（版本 %s）⇒ rc=2（判不了 ≠ 绿 · 照 ⑧b 同一条处置）\n", verDisp), 1000000, "      ")
		rc8d = 2
	} else {
		rc8dST, log8dST = pfCallGate(Z, "check-public-face-pairing", []string{"--self-test"})
		pfTail(stdout, log8dST, 2, "      ")
		if rc8dST != 0 {
			fmt.Fprintf(stdout, "   ⛔ ⑧d 的 must-fail 负控（该件 --self-test）不过（rc=%d）⇒ 判据自身没牙 ⇒ 记 BLOCKED\n", rc8dST)
		}
		rc8d, log8d = pfCallGate(Z, "check-public-face-pairing", []string{"--root", FACE, "--rev", base8d, "--no-self-test"})
		pfTail(stdout, log8d, 8, "      ")
	}

	// ⑧e 生成区一致（Go 半优先）
	fmt.Fprintln(stdout, "   ⑧e 生成区一致（新 · Go 半优先：zerg gate check gen-cli-reference --check → py 回退 → 两半皆缺 fail-closed · 明细 /tmp/pf8e.log）")
	rc8e, log8e := pfCallGate(Z, "gen-cli-reference", []string{"--check"})
	pfTail(stdout, log8e, 6, "      ")

	// 退码映射（只认 0/1/2；其它 ⇒ 2）。
	norm := func(rc int) int {
		switch rc {
		case 0, 1, 2:
			return rc
		default:
			return 2
		}
	}
	rc8a, rc8b, rc8c, rc8d, rc8e = norm(rc8a), norm(rc8b), norm(rc8c), norm(rc8d), norm(rc8e)
	if rc8cST != 0 {
		rc8c = 2
	}
	if rc8dST != 0 {
		rc8d = 2
	}
	r8Blocked := rc8a == 2 || rc8b == 2 || rc8c == 2 || rc8d == 2 || rc8e == 2
	r8TriFail := !r8Blocked && (rc8c != 0 || rc8d != 0 || rc8e != 0)
	if rc8a != 0 || rc8b != 0 {
		r8Note = 1
	}
	if r8Blocked {
		fmt.Fprintf(stdout, "   ⛔ 闸⑧ 判不了（rc=2 · ⑧a=%d ⑧b=%d ⑧c=%d ⑧d=%d ⑧e=%d）⇒ 记 BLOCKED（缺件 ≠ 绿 · 不许当绿）\n", rc8a, rc8b, rc8c, rc8d, rc8e)
		fmt.Fprintln(stdout, "      ★ **可接受失败只有一种 = `rc=2`**（缺件 / 判不了 = 「不给结论」）—— 照旧拦推送")
		blocked = 1
	} else if r8TriFail {
		fmt.Fprintf(stdout, "   ✗ 闸⑧ **阻断档**不过（⑧c=%d ⑧d=%d ⑧e=%d）⇒ **FAIL · 独立非零 · 不得折成 `rc=0`**\n", rc8c, rc8d, rc8e)
		if rc8e != 0 {
			fmt.Fprintln(stdout, "      ★ ⑧e 补救命令：`zerg gate check gen-cli-reference --emit`（改命令面必须同批重生成）")
		}
		fail = 1
	} else if r8RatchetRaised == 1 {
		fmt.Fprintf(stdout, "   ✗ 闸⑧ **存量债棘轮上调**（现读 %d 件 > 初值 %d 件）⇒ FAIL（只许降不许升）\n", r8Stale, publishPreflightRatchetInitial)
		fail = 1
	} else {
		if r8Note == 1 {
			fmt.Fprintf(stdout, "   △ 闸⑧ **报告档**有提示（⑧a=%d ⑧b=%d · 阻断档 ⑧c/⑧d/⑧e 全绿 ⇒ 报告档 = **REPORT**）\n", rc8a, rc8b)
			fmt.Fprintln(stdout, "      ★ 报告档**不是豁免档**：**不可接受三条** = 「本跑有提示」✗ /「已知落后一版」✗ /「只有 docs 面」✗")
			fmt.Fprintln(stdout, "      ★ 第二刀（⑧a/⑧b① 存量债升档）**本笔不落** —— 条件 = 该条现跑 `rc=0`")
		} else {
			r8StaleDisp := "?"
			if r8Stale >= 0 {
				r8StaleDisp = strconv.Itoa(r8Stale)
			}
			fmt.Fprintf(stdout, "   ✅ 闸⑧ **PASS**（无漂移 · 五条判据 ⑧a–⑧e 全绿 · 棘轮 %s ≤ %d）\n", r8StaleDisp, publishPreflightRatchetInitial)
		}
	}

	if fail != 0 {
		fmt.Fprintln(stdout, "❌ 发布前置未过 ⇒ 不得推送")
		return exitFail
	}
	if blocked != 0 {
		fmt.Fprintln(stdout, "⚠ 有闸「不给结论」（缺件/判不了 ⇒ BLOCKED）⇒ 也不得推送（rc=2）—— 补齐缺件后再跑")
		return exitBlocked
	}
	if r8Note != 0 {
		fmt.Fprintln(stdout, "✅ 八道阻断闸全过（闸⑧ 的**报告档**报了提示 · 阻断档 ⑧c/⑧d/⑧e 全绿 · 不拦推送）⇒ 允许推送")
	} else {
		fmt.Fprintln(stdout, "✅ 九道闸全过 ⇒ 允许推送")
	}
	return exitOK
}

// pfLeakScan —— 闸③ 旧壳内联 python 的逐字镜像：逐个提交 × 逐个私有串跑 `git grep -q -I`。
func pfLeakScan(D string) (int, string) {
	return pfLeakScanCapped(D, pfLeakScanCommitCap)
}

// pfLeakScanCapped —— pfLeakScan 的本体（cap 作参数 ⇒ 本单「超限」负控／夹具用；生产入口恒传 pfLeakScanCommitCap）。
func pfLeakScanCapped(D string, cap int) (int, string) {
	var b strings.Builder
	commits := []string{}
	if _, out := pfGitOut(D, "rev-list", "main"); out != "" {
		commits = strings.Fields(out)
	}
	// ★ 扫描上限（本单新增）：全史提交数超上限 ⇒ **不给结论**（判不了 ≠ 绿 ⇒ 交调用方记 BLOCKED · fail-closed）。
	if len(commits) > cap {
		fmt.Fprintf(&b, "   提交数=%d 超扫描上限 %d ⇒ 判不了（不给结论 · 不当绿）\n", len(commits), cap)
		return pfLeakScanOverCapRC, b.String()
	}
	pats := []string{"Mr2109", "Mr2109", "Mr2109", "/Volumes/" + "AHZ", "/Users/" + "Mr2109"}
	type hit struct{ pat, c string }
	bad := []hit{}
	for _, c := range commits {
		for _, p := range pats {
			rc, _ := pfGitOut(D, "grep", "-q", "-I", p, c, "--", ".")
			if rc == 0 {
				cc := c
				if len(cc) > 8 {
					cc = cc[:8]
				}
				bad = append(bad, hit{p, cc})
			}
		}
	}
	fmt.Fprintf(&b, "   提交数=%d 命中=%d（应 0）\n", len(commits), len(bad))
	for i, h := range bad {
		if i >= 5 {
			break
		}
		fmt.Fprintf(&b, "   ✗ ('%s', '%s')\n", h.pat, h.c)
	}
	if len(bad) > 0 {
		return 1, b.String()
	}
	return 0, b.String()
}

// pfStaleRow —— 判据① 一格（件 / git 最后提交日 / mtime / 判据取谁）。
type pfStaleRow struct {
	rel string
	g   int64
	mt  int64
	src string
}

// pfDocsFreshness —— 闸⑧b 内联判据（现值面 / 版本串 / 成对性）的逐字镜像。
// 返回 (rc, 输出, R8_STALE) —— `R8_STALE < 0` = 本跑**没打出**机读行（旧壳据此把 RC8b 记 2）。
func pfDocsFreshness(face string) (int, string, int) {
	var b strings.Builder
	const DOCS = pathPublishDocs
	docs := filepath.Join(face, DOCS)
	skipSuffix := []string{"~", ".bak", ".orig", ".rej", ".swp", ".pyc", ".DS_Store"}
	singleLang := []string{"publish/docs/design/"}
	stale := -1

	if fi, err := os.Stat(docs); err != nil || !fi.IsDir() {
		fmt.Fprintf(&b, "   ⛔ 缺件：%s/ 不在（缺件 ≠ 绿 ⇒ 不给结论）\n", DOCS)
		return 2, b.String(), stale
	}

	// os.walk(docs)（目录名字典序 · 跳 `.` 开头；文件名典序 · 跳 `.` 开头与 SKIP 后缀）。
	files := []string{}
	var walk func(dir string)
	walk = func(dir string) {
		ents, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		dirs := []string{}
		fns := []string{}
		for _, e := range ents {
			if e.IsDir() {
				dirs = append(dirs, e.Name())
			} else {
				fns = append(fns, e.Name())
			}
		}
		sort.Strings(dirs)
		sort.Strings(fns)
		for _, fn := range fns {
			if strings.HasPrefix(fn, ".") {
				continue
			}
			skip := false
			for _, s := range skipSuffix {
				if strings.HasSuffix(fn, s) {
					skip = true
					break
				}
			}
			if skip {
				continue
			}
			rel, err := filepath.Rel(face, filepath.Join(dir, fn))
			if err != nil {
				continue
			}
			files = append(files, filepath.ToSlash(rel))
		}
		for _, d := range dirs {
			if strings.HasPrefix(d, ".") {
				continue
			}
			walk(filepath.Join(dir, d))
		}
	}
	walk(docs)
	if len(files) == 0 {
		fmt.Fprintf(&b, "   ⛔ 空转：%s/ 下 0 件（「件没了所以全绿」是最难发现的那一类假绿）\n", DOCS)
		return 2, b.String(), stale
	}

	texts := map[string]string{}
	blocks := []string{}
	for _, rel := range files {
		raw, err := os.ReadFile(filepath.Join(face, rel))
		if err != nil {
			blocks = append(blocks, fmt.Sprintf("件读不了：%s（%v）", rel, err))
			continue
		}
		texts[rel] = string(raw)
	}
	if len(blocks) > 0 {
		for _, bl := range blocks {
			fmt.Fprintf(&b, "   ⛔ %s\n", bl)
		}
		fmt.Fprintf(&b, "   ⛔ 件读不了 %d 处 ⇒ **不给结论（rc=2）**（读不全就判不出真值）\n", len(blocks))
		return 2, b.String(), stale
	}

	// ── 版本落地日锚（现读）──
	ver := ""
	var anchor int64 = -1
	anchorSrc := ""
	vg := filepath.Join(face, "core/internal/version/version.go")
	if _, err := os.Stat(vg); err != nil {
		blocks = append(blocks, "缺件：core/internal/version/version.go（取不到版本真源 ⇒ 判不了「版本落地日」）")
	} else if raw, err := os.ReadFile(vg); err != nil {
		blocks = append(blocks, "缺件：core/internal/version/version.go（取不到版本真源 ⇒ 判不了「版本落地日」）")
	} else if m := pfVerRe.FindStringSubmatch(string(raw)); m == nil {
		blocks = append(blocks, "判不了：version.go 里读不出 `const Version`（拒绝猜）")
	} else {
		ver = m[1]
		if rc, so := pfGitOut(face, "log", "-1", "--format=%ct", "-S", `const Version = "`+ver+`"`, "--", "core/internal/version/version.go"); rc == 0 && pfIsDigits(so) {
			v, _ := strconv.ParseInt(so, 10, 64)
			anchor, anchorSrc = v, "该版本行所在提交"
		} else if rc2, so2 := pfGitOut(face, "log", "-1", "--format=%ct", "v"+ver); rc2 == 0 && pfIsDigits(so2) {
			v, _ := strconv.ParseInt(so2, 10, 64)
			anchor, anchorSrc = v, "tag v"+ver
		} else {
			blocks = append(blocks, fmt.Sprintf("判不了：取不到版本 %s 的落地日（无该版本行的提交、也无 tag）", ver))
		}
	}

	fails := []string{}
	late := 0
	rows := []pfStaleRow{}
	if anchor >= 0 {
		// 脏件集（未跟踪 / 已改动 ⇒ 用 mtime 兜底）。
		dirty := map[string]bool{}
		if _, so := pfGitOut(face, "status", "--porcelain", "--", DOCS); true {
			for _, ln := range strings.Split(so, "\n") {
				if strings.TrimSpace(ln) == "" || len(ln) < 3 {
					continue
				}
				p := strings.TrimSpace(ln[3:])
				if i := strings.Index(p, " -> "); i >= 0 {
					p = strings.TrimSpace(p[i+4:])
				}
				dirty[strings.Trim(p, `"`)] = true
			}
		}
		// git 最后提交日（newest-first ⇒ 首次出现即最近）。
		last := map[string]int64{}
		if _, so := pfGitOut(face, "log", "--format=C%ct", "--name-only", "--", DOCS); true {
			var cur int64 = -1
			for _, ln := range strings.Split(so, "\n") {
				ln = strings.TrimSpace(ln)
				if ln == "" {
					continue
				}
				if strings.HasPrefix(ln, "C") && pfIsDigits(ln[1:]) {
					v, _ := strconv.ParseInt(ln[1:], 10, 64)
					cur = v
				} else if cur >= 0 {
					if _, ok := last[ln]; !ok {
						last[ln] = cur
					}
				}
			}
		}
		for _, rel := range files {
			var mt int64
			if fi, err := os.Stat(filepath.Join(face, rel)); err == nil {
				mt = fi.ModTime().Unix()
			} else {
				blocks = append(blocks, fmt.Sprintf("判不了：%s 的时间读不出（%v）", rel, err))
				continue
			}
			g := last[rel]
			eff := int64(0)
			src := ""
			if dirty[rel] || g == 0 {
				eff, src = pfMax64(g, mt), "mtime"
			} else {
				eff, src = g, "git"
			}
			if eff < anchor {
				rows = append(rows, pfStaleRow{rel, g, mt, src})
			} else {
				late++
			}
		}
		fmt.Fprintf(&b, "   ① 现值面：件 %d · 版本真源 %s · 落地日 %s（%s）· 迟于该日 %d 件 · **早于该日 %d 件**\n", len(files), ver, pfTS(anchor), anchorSrc, late, len(rows))
		fmt.Fprintf(&b, "R8_STALE=%d\n", len(rows))
		fmt.Fprintf(&b, "R8_STALE_TOTAL=%d\n", len(files))
		stale = len(rows)
		if len(rows) > 0 {
			fmt.Fprintln(&b, "      陈旧（逐件点名 · 读数 git=最后提交日 / mtime=文件时间）：")
			n := len(rows)
			if n > 10 {
				n = 10
			}
			for _, r := range rows[:n] {
				fmt.Fprintf(&b, "        ── %s   git=%s · mtime=%s（判据取 %s）\n", r.rel, pfTS(r.g), pfTS(r.mt), r.src)
			}
			if len(rows) > 10 {
				fmt.Fprintf(&b, "        …另有 %d 件（本节只列前 10 件）\n", len(rows)-10)
			}
			fails = append(fails, fmt.Sprintf("现值面：%d 件早于版本落地日（`%s/` 没跟上本版）", len(rows), DOCS))
		}
	} else if len(blocks) == 0 {
		blocks = append(blocks, "判不了：版本落地日为空")
	}

	// ── 判据② 版本串 `2.5.`（前缀）──
	vh := []string{}
	for _, rel := range files {
		for i, ln := range strings.Split(texts[rel], "\n") {
			if strings.Contains(ln, pfDocsTOK) {
				vh = append(vh, fmt.Sprintf("%s:%d", rel, i+1))
			}
		}
	}
	if len(vh) > 0 {
		head := vh
		if len(head) > 5 {
			head = head[:5]
		}
		fails = append(fails, fmt.Sprintf("版本串 `2.5.` 命中 %d 处（须 0）：%s", len(vh), strings.Join(head, " · ")))
	} else {
		fmt.Fprintln(&b, "   ② 版本串 `2.5.` 命中 **0** 处（须 0 · 判据是前缀不是三段正则 ⇒ 不撞 `192.0.2.10`）")
	}

	// ── 判据③ 成对性（中英成对 · 现测）──
	partner := func(rel string) (string, bool) {
		d := path.Dir(rel)
		bn := path.Base(rel)
		if bn == "README.md" || bn == "README.zh-CN.md" {
			if bn == "README.zh-CN.md" {
				return d + "/README.md", true
			}
			return d + "/README.zh-CN.md", true
		}
		if strings.HasSuffix(bn, ".zh-CN.md") {
			return d + "/" + bn[:len(bn)-len(".zh-CN.md")] + ".en.md", true
		}
		if strings.HasSuffix(bn, ".en.md") {
			return d + "/" + bn[:len(bn)-len(".en.md")] + ".zh-CN.md", true
		}
		return "", false
	}
	pairs := map[string]string{}
	single := []string{}
	unpaired := []string{}
	nonmd := 0
	for _, rel := range files {
		if !strings.HasSuffix(rel, ".md") {
			nonmd++
			continue
		}
		isSingle := false
		for _, p := range singleLang {
			if strings.HasPrefix(rel, p) {
				isSingle = true
				break
			}
		}
		if isSingle {
			single = append(single, rel)
			continue
		}
		q, ok := partner(rel)
		if !ok {
			unpaired = append(unpaired, rel)
			continue
		}
		if fi, err := os.Stat(filepath.Join(face, q)); err != nil || fi.IsDir() {
			unpaired = append(unpaired, rel)
			continue
		}
		if strings.HasSuffix(rel, ".en.md") {
			pairs[q] = rel
		} else if strings.HasSuffix(rel, "README.md") && !strings.HasSuffix(rel, "zh-CN.md") {
			pairs[q] = rel
		} else {
			pairs[rel] = q
		}
	}
	short := []string{}
	for k := range pairs {
		s := path.Base(k)
		s = strings.Replace(s, ".zh-CN.md", "", 1)
		s = strings.Replace(s, ".md", "", 1)
		short = append(short, s)
	}
	sort.Strings(short)
	if len(unpaired) > 0 {
		u := append([]string{}, unpaired...)
		sort.Strings(u)
		if len(u) > 6 {
			u = u[:6]
		}
		fails = append(fails, fmt.Sprintf("成对性：**%d 件**没有对应语言件：%s", len(unpaired), strings.Join(u, " · ")))
	} else {
		sb := []string{}
		for _, s := range single {
			sb = append(sb, path.Base(s))
		}
		sort.Strings(sb)
		fmt.Fprintf(&b, "   ③ 成对性：中英成对 **%d 对**齐 ✓（%s）· 登记的单语例外 %d 件（%s · 理由：设计稿理念件，映射只有 zh 一条 · `T-11b` **已正式登记** 2026-09-24）· 非 .md 件 %d（不判成对）\n",
			len(pairs), strings.Join(short, " "), len(single), strings.Join(sb, " · "), nonmd)
	}

	// ── 三档出口 ──
	if len(blocks) > 0 {
		for _, bl := range blocks {
			fmt.Fprintf(&b, "   ⛔ %s\n", bl)
		}
		fmt.Fprintf(&b, "   ⛔ 缺件/判不了 %d 处 ⇒ **不给结论（rc=2）**（缺件 ≠ 绿）\n", len(blocks))
		return 2, b.String(), stale
	}
	if len(fails) > 0 {
		for _, f := range fails {
			fmt.Fprintf(&b, "   ✗ %s\n", f)
		}
		fmt.Fprintf(&b, "⇒ **有红（rc=1）**：三条判据里 %d 条不过（**存量债档 + 报告档** ⇒ 不拦推送 · 但必须看得见）\n", len(fails))
		return 1, b.String(), stale
	}
	fmt.Fprintln(&b, "⇒ 全绿（rc=0）：现值面 / 版本串 / 成对性三条判据都过")
	return 0, b.String(), stale
}
