// family_publish_public_native.go —— `zerg publish run` 的 **Go 真身**（发布链 Go 半优先 · 本批第二件）。
//
// 病（原壳）：`cmdPublishRun`（family_publish_run.go）只 `bash scripts/build/publish-public.sh`
// —— 压平快照发布器的**九段编排**（环境检查 / 导出白名单树 / 套发布件 / 快照变换 / 门禁扫描 /
// 压平提交 / 推送）长在 shell 里，CLI 面看不见真身。
//
// 本笔把编排**搬进 CLI 进程内**，逐段与旧壳 `scripts/build/publish-public.sh`逐字对齐：
//
//	① 环境检查（缺 wl/rules、缺 rsync/python3、私有 HEAD、输出目录）
//	② EXCLUDES 双源对拍闸（新真源 `publish/excludes.txt` ⟷ 旧壳内联 `EXCLUDES=(…)` 数组）
//	③ 导出白名单文件树（HEAD 树挑件 → 逐 blob 读 → 排除项 rm → 噪声清扫）
//	④ 套发布专属文件（publish/ → 快照）
//	⑤ 快照变换（Cargo.toml / Cargo.lock 跨仓解依赖 + 替换规则）
//	⑥ 门禁扫描（shell unicode-vars → 词表扫描 → `_INL_BLOCK` 真源扫 → 私有面 → gitleaks）
//	⑦ 文档 / i18n / 版本号 / 兼容清单四道随版门
//	⑧ 单个压平提交（SQUASH）
//	⑨ --push 才推公开仓（默认只准备、**不出网**）
//
// ★ 两态对拍口径：`zerg publish run [--out D] …`（本真身）⟷ `bash scripts/build/publish-public.sh [--out D]`
//
//	（旧壳）在**同一棵仓外夹具**上 **rc / stdout / stderr 三面逐字同**（掩时间戳 / 临时路径）。
//
// ★ 数据处理面（5 段内联 python：白名单挑选 / Cargo.toml / Cargo.lock / 替换规则 / 词表扫描）
//
//	**逐字节照交 `python3 -`**（`family_publish_public_pyblocks.go` 的常量 —— base64url 承载，
//	经 `ppPyDecode` 解回**逐字节同**的原文，与旧壳同一条子进程边界）
//	⇒ 这几段（含 `os.walk` 序 / `re` 语义 / unicode 转义）**保证与旧壳逐字同**，不另造 Go 第二口径。
//
// ★ 两处**真源**改用**仓内既有 Go 真身**（✗ 不自造第二份名单）：
//
//	· EXCLUDES 排除表命中 ⇒ `loadExcludesTable`（`excludes_rules.go` 的**唯一** Go 解析件）；
//	· `_INL_BLOCK` 内容扫 ⇒ `chsecBlockSpecs`（`check_history_secrets.go` 里与镜像器
//	  `publish/mirror-public-lib.py` 的 `_INL_BLOCK` **逐字同源**的 Go 镜像）+ `chsecMask`。
//
// ★ 各判据门**按名调用同族既有 Go 判据体**（`gateCheckGoEntry` · 经 `gateCheckRunInproc`
//
//	走与 `zerg gate check <名>` 同一条进程内入口）：chdir 到仓根 + 注入 ZERG_REPO/ZERG_REPO_ROOT。
//	本件**不复制任何判据实现**、不改任何闸的退码。
//
// ★ 未覆盖面（✗ 不声称逐字对齐）：
//
//	· 旧壳 `-h/--help` 走 `sed -n '2,33p' "$0"`（打印脚本自身头注释）—— 命令面 `--help` 走 CLI 用法树，不同源；
//	· 旧壳未知参数退 2 的正文 ⟷ CLI 解析器退码/正文不同源；
//	· `os.walk` 序与 Go 自写 walk（`Readdir(-1)`）在**符号链接目录** / 非 UTF-8 文本 decode 粒度上可能不同；
//	· `python3` 解释器版本随 PATH 变（库文案跨版本会变的只有 argparse，本件未用）。
//
// ★ 与 `publish/mirror-public-lib.py` 的**共用逻辑**（供下一件两器批用）：两器都吃 `publish/excludes.txt`
//
//	（镜像器 `Assets.exclude_kind` 的 `EXCLUDES` 层 ⟷ 本件 `loadExcludesTable`）、都吃
//	`publish/mirror-public-lib.py` 的 `_INL_BLOCK`（镜像器 `Assets.scan` ⟷ 本件 `chsecBlockSpecs` 同源镜像）、
//	都吃同一套替换规则 / 白名单 / 私有面黑名单 —— 单一真源、无第二份名单。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// ppRunOpts —— `zerg publish run` 的入口参数（由 `cmdPublishRun` 解析后传入）。
type ppRunOpts struct {
	Out         string // --out 落点（空 ⇒ /tmp/zerg-public-<TS>）
	Push        bool   // --push
	DryRun      bool   // --dry-run（旧壳语义：跑到 gitleaks 停在 git 之前 · ✗ 不建提交）
	Remote      string // --remote / ZERG_PUBLIC_REMOTE
	CheckParity bool   // --check-excludes-parity（只跑对拍闸 · 不导出）
}

// ppEXCLUDESRe —— 旧壳内联数组块的正则（逐字照旧壳 :104 `re.search(r"(?ms)^EXCLUDES=\((.*?)\n\)")`）。
var ppEXCLUDESRe = regexp.MustCompile(`(?ms)^EXCLUDES=\((.*?)\n\)`)

// ppSevenDigitRe —— 旧壳噪声清扫里 `-name '*.[0-9][0-9][0-9][0-9][0-9][0-9][0-9]*'` 的 Go 口径。
var ppSevenDigitRe = regexp.MustCompile(`\.[0-9]{7}`)

// ppRCNotFound —— shell 惯例退码 127（子进程起不来）；127 属 internal 命名空间、不入命令面真源表（先例 GAP-20261002-08）。
const ppRCNotFound = 127

// ppSay —— 复刻旧壳 `say()`：`printf '\n\033[1m== %s\033[0m\n' "$*"`（stdout）。
func ppSay(w io.Writer, s string) {
	fmt.Fprintf(w, "\n\x1b[1m== %s\x1b[0m\n", s)
}

// ppDeprecation —— 复刻旧壳 `deprecation_warning()`（3 行 · 全部 stderr · 首行带前导换行、末行带尾空行）。
func ppDeprecation(stderr io.Writer) {
	fmt.Fprint(stderr, "\n\x1b[1;33m⚠️  已弃用：scripts/build/publish-public.sh（压平快照发布器）\x1b[0m\n")
	fmt.Fprint(stderr, "\x1b[1;33m   → 已被 publish/mirror-public.sh（逐提交镜像）取代；正式发布请走镜像器。\x1b[0m\n")
	fmt.Fprint(stderr, "\x1b[1;33m   → 计划在**下一个版本（v2.6.0）**删除；现仅作应急后备与两器对照保留。\x1b[0m\n\n")
}

// ppRunChild —— 起一个子进程（登记 `runningChild` 以便收尾杀整组），返回其进程退码。
func ppRunChild(cmd *exec.Cmd) int {
	runningChild = cmd
	defer func() { runningChild = nil }()
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode()
		}
		return ppRCNotFound
	}
	return 0
}

// ppPython —— 复刻旧壳 `python3 - <args> <<'PY'`（脚本经 **stdin** 交过去 · cwd 继承进程 cwd）。
func ppPython(script string, args []string, stdout, stderr io.Writer) int {
	cmd := armChildGroup(exec.Command("python3", append([]string{"-"}, args...)...))
	cmd.Stdin = strings.NewReader(script)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return ppRunChild(cmd)
}

// ppPrivatePathPatternsJSON —— 私有路径**检测模式**的真源归位（件 = family_publish_public_pyblocks.go 的
// `ppPyPatterns` 内嵌 python 的 PATTERNS 表第 3/4 条）。
//
// 真源 = `_INL_BLOCK`（`publish/mirror-public-lib.py`）的 **Go 镜像** `chsecBlockSpecs`
// （`check_history_secrets.go`）—— 本件既有「4.1 BLOCK 内容扫描」（`ppInlineBlockScan`）已把它当真源用。
// 取其中「私有卷」「家目录」两条的模式串，序 = [私有卷, 家目录]（对齐旧内嵌表位置）⇒ 交 `python3 -`
// 的 argv[2]（JSON 数组）。✗ 本件源码不写私有字面 ⇒ `check-hardcoded-private-paths` 对 `core/` 零命中。
// 取不到（真源缺条）⇒ ok=false（调用方 fail-closed，✗ 不静默拿空串扫面 —— `re.search("")` 会命中每一行）。
func ppPrivatePathPatternsJSON() (string, bool) {
	vol, home := "", ""
	for _, sp := range chsecBlockSpecs {
		if vol == "" && strings.Contains(sp.why, "私有卷") {
			vol = sp.pat
		}
		if home == "" && strings.Contains(sp.why, "家目录") {
			home = sp.pat
		}
	}
	if vol == "" || home == "" {
		return "", false
	}
	b, err := json.Marshal([]string{vol, home})
	if err != nil {
		return "", false
	}
	return string(b), true
}

// ppGit —— 仓内 git（继承 cwd）；返回 rc、stdout（**去尾空白**，照 `$(…)`）。
func ppGit(dir string, args ...string) (int, string) {
	var out strings.Builder
	cmd := armChildGroup(exec.Command("git", args...))
	cmd.Dir = dir
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	rc := ppRunChild(cmd)
	return rc, strings.TrimSpace(out.String())
}

// ppGitRun —— 仓内 git，stdout/stderr 直通给定流（用于可见输出的 git 子命令）。
func ppGitRun(dir string, stdout, stderr io.Writer, env []string, args ...string) int {
	cmd := armChildGroup(exec.Command("git", args...))
	cmd.Dir = dir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}
	return ppRunChild(cmd)
}

// ppExistsFile / ppExistsDir —— 存在性（照旧壳 `[ -f ]` / `[ -d ]`）。
func ppExistsFile(p string) bool { fi, err := os.Stat(p); return err == nil && !fi.IsDir() }
func ppExistsDir(p string) bool  { fi, err := os.Stat(p); return err == nil && fi.IsDir() }

// ppCopyFile —— 复刻 `cp -p`（保留可执行位；✗ 不复刻时间戳，git 不记 mtime）。
func ppCopyFile(src, dst string) error {
	fi, err := os.Stat(src)
	if err != nil {
		return err
	}
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dst, b, fi.Mode().Perm()); err != nil {
		return err
	}
	return os.Chmod(dst, fi.Mode().Perm())
}

// ppGlobBash —— 复刻 bash 无引号 glob：无命中时保留**字面词**（✗ 不退化成空 • set -e 下旧壳如此）。
func ppGlobBash(pattern string) []string {
	ms, err := filepath.Glob(pattern)
	if err != nil || len(ms) == 0 {
		return []string{pattern}
	}
	return ms
}

// ppCountFiles —— 复刻 `find "$OUT" -type f | wc -l`。
func ppCountFiles(root string) int {
	n := 0
	_ = filepath.Walk(root, func(_ string, fi os.FileInfo, err error) error {
		if err == nil && fi.Mode().IsRegular() {
			n++
		}
		return nil
	})
	return n
}

// ppCleanup —— 复刻旧壳尾部 `find … -delete` 噪声清扫（顺序不影响终树）。
func ppCleanup(out string) {
	rmIf := func(p string) { _ = os.RemoveAll(p) }
	_ = filepath.Walk(out, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		name := fi.Name()
		if fi.IsDir() {
			if name == "__pycache__" {
				rmIf(p)
				return filepath.SkipDir
			}
			return nil
		}
		if !fi.Mode().IsRegular() {
			return nil
		}
		if strings.HasSuffix(name, ".orig") || strings.HasSuffix(name, ".rej") || strings.HasSuffix(name, ".bak") {
			rmIf(p)
			return nil
		}
		if ppSevenDigitRe.MatchString(name) {
			rmIf(p)
			return nil
		}
		switch filepath.Ext(name) {
		case ".pyc", ".pyo", ".o", ".a", ".so", ".dylib", ".class":
			rmIf(p)
		}
		return nil
	})
}

// ppWalkPython —— 复刻 python `os.walk` 的**遍历序**（scandir / readdir 序 · ✗ 不排序）与 dir 过滤。
// 返回 root 下所有文件的**仓内相对路径**（slash）。skipDirs 命中即整目录跳过（照 `dirs[:] = …`）。
func ppWalkPython(root string, skipDirs []string) []string {
	skip := map[string]bool{}
	for _, d := range skipDirs {
		skip[d] = true
	}
	var files []string
	var walk func(dir string)
	walk = func(dir string) {
		f, err := os.Open(dir)
		if err != nil {
			return
		}
		infos, err := f.Readdir(-1)
		_ = f.Close()
		if err != nil {
			return
		}
		var dirs []string
		for _, fi := range infos {
			name := fi.Name()
			if fi.IsDir() {
				if skip[name] {
					continue
				}
				dirs = append(dirs, name)
				continue
			}
			if rel, rerr := filepath.Rel(root, filepath.Join(dir, name)); rerr == nil {
				files = append(files, filepath.ToSlash(rel))
			}
		}
		for _, d := range dirs {
			walk(filepath.Join(dir, d))
		}
	}
	walk(root)
	return files
}

// ppExcludesParity —— 复刻旧壳 `excludes_parity_gate()`（新真源 `publish/excludes.txt` ⟷ 旧壳内联 `EXCLUDES` 数组）。
// 逐字对齐旧壳 :90-124 的 python（含 rc 与逐行正文）；不等 ⇒ rc=2（fail-closed）。
func ppExcludesParity(root string, stderr io.Writer) int {
	tbl := filepath.Join(root, "publish", "excludes.txt")
	sh := filepath.Join(root, "scripts", "build", "publish-public.sh")
	tb, terr := os.ReadFile(tbl)
	if terr != nil {
		fmt.Fprintf(stderr, "对拍闸 ✗ 缺新真源 %s ⇒ 不给结论 rc=2\n", tbl)
		return exitUsage
	}
	newList := []string{}
	for _, ln := range strings.Split(string(tb), "\n") {
		s := strings.TrimSpace(ln)
		if s != "" && !strings.HasPrefix(s, "#") {
			newList = append(newList, s)
		}
	}
	shb, serr := os.ReadFile(sh)
	m := ppEXCLUDESRe.FindStringSubmatch(string(shb))
	if serr != nil || m == nil {
		fmt.Fprintf(stderr, "对拍闸 ✗ %s 里找不到排除数组块 ⇒ 不给结论 rc=2\n", sh)
		return exitUsage
	}
	oldList := []string{}
	for _, raw := range strings.Split(m[1], "\n") {
		ln := strings.TrimSpace(raw)
		if ln == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		ln = strings.TrimSpace(strings.SplitN(ln, "#", 2)[0])
		ln = strings.Trim(strings.Trim(ln, "\""), "'")
		if ln != "" {
			oldList = append(oldList, ln)
		}
	}
	if len(newList) == 0 || len(oldList) == 0 {
		fmt.Fprintf(stderr, "对拍闸 ✗ 解析出空表（新表 %d / 旧数组 %d）⇒ 不给结论 rc=2\n", len(newList), len(oldList))
		return exitUsage
	}
	if len(newList) != len(oldList) {
		fmt.Fprintf(stderr, "对拍闸 ✗ 条数不等：新表 %d ≠ 旧数组 %d（%s ⟷ %s）⇒ rc=2\n", len(newList), len(oldList), tbl, sh)
		return exitUsage
	}
	for i := range newList {
		if newList[i] != oldList[i] {
			fmt.Fprintf(stderr, "对拍闸 ✗ 第 %d 条不等：新表 %s ≠ 旧数组 %s ⇒ rc=2\n",
				i+1, ppPyRepr(newList[i]), ppPyRepr(oldList[i]))
			return exitUsage
		}
	}
	fmt.Fprintf(stderr, "对拍闸 ✓ 逐条相等：%s（%d 条）⟷ %s 旧数组（%d 条）\n", tbl, len(newList), sh, len(oldList))
	return exitOK
}

// ppPyRepr —— 近似 python `repr(str)`（`'…'` 单引号）；普通 ASCII 路径逐字同，含引号/控制符时**不保证**
// （见未覆盖面对）。仅用于对拍闸的「第 N 条不等」正文。
func ppPyRepr(s string) string {
	r := strings.NewReplacer("\\", "\\\\", "'", "\\'", "\n", "\\n", "\t", "\\t", "\r", "\\r")
	return "'" + r.Replace(s) + "'"
}

// ppInlineBlockScan —— 复刻旧壳 :1101-1130 的 BLOCK 内容扫（真源 `_INL_BLOCK`）。
// ★ 真源用仓内 Go 镜像 `chsecBlockSpecs`（与 `publish/mirror-public-lib.py::_INL_BLOCK` **逐字同源**）；
// 口径同镜像器 `Assets.scan`：只扫**可 UTF-8 解码**的文本件。命中 ⇒ rc=1（整批中止）。
func ppInlineBlockScan(out string, stdout io.Writer) int {
	type hit struct{ rel, why, sample string }
	var hits []hit
	for _, rel := range ppWalkPython(out, []string{".git", "target", "node_modules"}) {
		raw, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(rel)))
		if err != nil || !utf8.Valid(raw) {
			continue
		}
		txt := string(raw)
		for _, sp := range chsecBlockSpecs {
			rx, cerr := regexp.Compile(sp.pat)
			if cerr != nil {
				continue
			}
			if m := rx.FindString(txt); m != "" {
				hits = append(hits, hit{rel, sp.why, chsecMask(m)})
				break
			}
		}
	}
	fmt.Fprintf(stdout, "  BLOCK 内容扫描（真源 _INL_BLOCK · %d 条）：命中 %d 处\n", len(chsecBlockSpecs), len(hits))
	for i, h := range hits {
		if i >= 40 {
			break
		}
		fmt.Fprintf(stdout, "    ❌ %s  %s\n       ← %s\n", h.why, h.rel, h.sample)
	}
	if len(hits) > 40 {
		fmt.Fprintf(stdout, "    …（其余 %d 处省略）\n", len(hits)-40)
	}
	if len(hits) > 0 {
		return exitFail
	}
	return exitOK
}

// ppGate —— 按名调用同族既有 Go 判据体（进程内 · 与 `zerg gate check <名>` 同一条入口）；
// 返回 rc。stdout/stderr 可分别给（✗ 不复制判据实现）。
func ppGate(root, name string, stdout, stderr io.Writer, argv []string) int {
	fn, ok := gateCheckGoEntry[name]
	if !ok {
		return exitBlocked
	}
	return gateCheckRunInproc(fn, root, stdout, stderr, argv)
}

// publishPublicNative —— `zerg publish run` 的进程内真身（旧壳九段 · 逐字对齐）。
func publishPublicNative(inv *invocation, stdout, stderr io.Writer, opts ppRunOpts) int {
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 跑不动发布（不给结论 · 退码 8）\n", progName)
		return exitBlocked
	}
	whitelist := filepath.Join(root, "publish", "whitelist.txt")
	rules := filepath.Join(root, "publish", "replace-rules.tsv")

	// deprecation_warning（旧壳 :52 —— 先于一切后续动作）。
	ppDeprecation(stderr)

	out := opts.Out
	if out == "" {
		out = "/tmp/zerg-public-" + time.Now().Format("20060102-150405")
	}
	authorName := ppEnvOr("SNAPSHOT_AUTHOR_NAME", "The Zerg Swarm Authors")
	authorEmail := ppEnvOr("SNAPSHOT_AUTHOR_EMAIL", "47194037+Mr2109@users.noreply.github.com")

	// ── 0/6 环境检查 ──
	ppSay(stdout, "0/6 环境检查")
	if !ppExistsFile(whitelist) {
		fmt.Fprintf(stderr, "缺少 %s\n", whitelist)
		return exitFail
	}
	if !ppExistsFile(rules) {
		fmt.Fprintf(stderr, "缺少 %s\n", rules)
		return exitFail
	}
	if _, err := exec.LookPath("rsync"); err != nil {
		fmt.Fprintln(stderr, "需要 rsync")
		return exitFail
	}
	if _, err := exec.LookPath("python3"); err != nil {
		fmt.Fprintln(stderr, "需要 python3（用于替换规则与扫描）")
		return exitFail
	}
	privHead := "unknown"
	if rc, o := ppGit(root, "rev-parse", "HEAD"); rc == 0 && o != "" {
		privHead = o
	}
	fmt.Fprintf(stdout, "私有仓 HEAD: %s\n", ppFirst12(privHead))
	fmt.Fprintf(stdout, "输出目录:    %s\n", out)

	// ── 对拍闸（迁表步 · 双源窗）──
	if rc := ppExcludesParity(root, stderr); rc != 0 {
		return rc
	}
	if opts.CheckParity {
		ppSay(stdout, "对拍闸：--check-excludes-parity（只跑本闸 · 不导出）")
		return exitOK
	}

	// ── 1/6 导出白名单文件树（★ 源面 = HEAD 树 · 与镜像器 `publish/mirror-public-lib.py` 的
	//   `ls-tree -r HEAD` 及旧壳 `scripts/build/publish-public.sh` 逐字同源：件集与内容都取已提交 HEAD
	//   面；工作树的未提交改动不再被带出。内嵌 python 块与旧壳 heredoc 逐字节同、逐 blob 写入。）──
	ppSay(stdout, "1/6 导出白名单文件树")
	_ = os.RemoveAll(out)
	_ = os.MkdirAll(out, 0o755)
	listPath := out + ".list"
	if rc := ppPython(ppPyDecode(ppPyWhitelistB64), []string{root, whitelist, listPath, out}, stdout, stderr); rc != 0 {
		return exitFail
	}
	// 排除项（真源 publish/excludes.txt · 唯一 Go 解析件 loadExcludesTable；命中即 rm -rf）。
	entries, eerr := loadExcludesTable(filepath.Join(root, "publish", "excludes.txt"))
	if eerr != nil {
		fmt.Fprintf(stderr, "%s: 读不到排除表 publish/excludes.txt ⇒ 不给结论\n", progName)
		return exitBlocked
	}
	for _, e := range entries {
		_ = os.RemoveAll(filepath.Join(out, filepath.FromSlash(e)))
	}
	ppCleanup(out)
	fmt.Fprintf(stdout, "导出文件数: %d\n", ppCountFiles(out))

	// ── 2/6 套用发布专属文件（publish/ → 快照）──
	ppSay(stdout, "2/6 套用发布专属文件（publish/ → 快照）")
	ppCopyFile(filepath.Join(root, "publish", "LICENSE"), filepath.Join(out, "LICENSE"))
	ppCopyFile(filepath.Join(root, "publish", "NOTICE"), filepath.Join(out, "NOTICE"))
	ppCopyFile(filepath.Join(root, "publish", "THIRD_PARTY_LICENSES.md"), filepath.Join(out, "THIRD_PARTY_LICENSES.md"))
	ppCopyFile(filepath.Join(root, "publish", "CONTRIBUTING.md"), filepath.Join(out, "CONTRIBUTING.md"))
	ppCopyFile(filepath.Join(root, "publish", "SECURITY.md"), filepath.Join(out, "SECURITY.md"))
	if ppExistsFile(filepath.Join(root, ".env.example")) {
		ppCopyFile(filepath.Join(root, ".env.example"), filepath.Join(out, ".env.example"))
	}
	ppCopyFile(filepath.Join(root, "publish", "config.example.yaml"), filepath.Join(out, "gateway", "fleet.example.yaml"))
	if ppExistsFile(filepath.Join(root, "publish", "gitleaks.toml")) {
		ppCopyFile(filepath.Join(root, "publish", "gitleaks.toml"), filepath.Join(out, ".gitleaks.toml"))
	}
	ppCopyFile(filepath.Join(root, "publish", "install.sh"), filepath.Join(out, "install.sh"))
	if ppExistsDir(filepath.Join(root, "publish", "docs")) {
		_ = os.MkdirAll(filepath.Join(out, "docs", "design"), 0o755)
		for _, f := range ppGlobBash(filepath.Join(root, "publish", "docs", "*.md")) {
			if !ppExistsFile(f) {
				continue
			}
			b := filepath.Base(f)
			if b == "README.md" || b == "README.zh-CN.md" {
				ppCopyFile(f, filepath.Join(out, b))
			} else {
				ppCopyFile(f, filepath.Join(out, "docs", b))
			}
		}
		for _, f := range ppGlobBash(filepath.Join(root, "publish", "docs", "design", "*.md")) {
			if !ppExistsFile(f) {
				continue
			}
			ppCopyFile(f, filepath.Join(out, "docs", "design", filepath.Base(f)))
		}
		fmt.Fprintf(stdout, "  公开文档: %d 篇（含 README）\n", ppCountMatching(filepath.Join(out, "docs"), ".md"))
	}
	_ = os.MkdirAll(filepath.Join(out, ".github", "workflows"), 0o755)
	ppCopyFile(filepath.Join(root, "publish", "ci", "ci.yml"), filepath.Join(out, ".github", "workflows", "ci.yml"))
	ppCopyFile(filepath.Join(root, "publish", "ci", "release-agent.yml"), filepath.Join(out, ".github", "workflows", "release-agent.yml"))
	_ = os.MkdirAll(filepath.Join(out, "docs", "design"), 0o755)
	if ppExistsFile(filepath.Join(out, "docs", "skills", "tool-upgrade.md")) {
		_ = os.Rename(filepath.Join(out, "docs", "skills", "tool-upgrade.md"), filepath.Join(out, "docs", "design", "工具升级规范.md"))
	}
	_ = os.RemoveAll(filepath.Join(out, "publish"))

	// ── 3/6 快照变换（跨仓依赖解除 + 替换规则）──
	ppSay(stdout, "3/6 快照变换（跨仓依赖解除 + 替换规则）")
	if ppExistsFile(filepath.Join(out, "ui", "Cargo.toml")) {
		ppPython(ppPyDecode(ppPyCargoTomlB64), []string{filepath.Join(out, "ui", "Cargo.toml")}, stdout, stderr)
	}
	if ppExistsFile(filepath.Join(out, "ui", "Cargo.lock")) {
		ppPython(ppPyDecode(ppPyCargoLockB64), []string{filepath.Join(out, "ui", "Cargo.lock")}, stdout, stderr)
	}
	if rc := ppPython(ppPyDecode(ppPyReplaceRulesB64), []string{out, rules}, stdout, stderr); rc != 0 {
		return exitFail
	}

	// ── 4/6 门禁扫描（命中即中止）──
	ppSay(stdout, "4/6 门禁扫描（命中即中止）")
	// 4.0 shell 静态检查（$VAR 后紧跟非 ASCII）：按名调用 Go 判据体（`check-shell-unicode-vars`）。
	suvFiles := append(ppGlobBash(filepath.Join(out, "scripts", "*.sh")), ppGlobBash(filepath.Join(out, "*.sh"))...)
	suvArgv := append([]string{"--check"}, suvFiles...)
	if rc := ppGate(root, "check-shell-unicode-vars", stdout, stderr, suvArgv); rc != 0 {
		fmt.Fprint(stderr, "❌ 脚本里有 $VAR 后紧跟非 ASCII 的坑——修完再发（zerg gate check check-shell-unicode-vars --check <files> 可就地修）\n")
		return exitFail
	}
	// 4.0b 词表扫描（纯数据面 · 逐字节交 python3）。
	patJSON, patOK := ppPrivatePathPatternsJSON()
	if !patOK {
		fmt.Fprint(stderr, "  ❌ 词表扫描真源取不到私有路径模式（chsecBlockSpecs 缺「私有卷」/「家目录」）—— 已中止导出（一个字节都不推）\n")
		return exitFail
	}
	if rc := ppPython(ppPyDecode(ppPyPatternsB64), []string{out, patJSON}, stdout, stderr); rc != 0 {
		return exitFail
	}
	// 4.1 BLOCK 内容扫描（真源 _INL_BLOCK · Go 镜像）。
	if rc := ppInlineBlockScan(out, stdout); rc != 0 {
		fmt.Fprint(stderr, "❌ BLOCK 内容扫描命中（真源 _INL_BLOCK）—— 已中止导出（一个字节都不推，同镜像器 Q3 口径）\n")
		return exitFail
	}
	fmt.Fprintln(stdout, "  BLOCK 内容扫描: 通过")
	// 4a 私有面门禁（按名调用 Go 判据体 `check-public-tree-private`）。
	privRC := ppGate(root, "check-public-tree-private", stdout, stderr, []string{out})
	if privRC == 0 {
		fmt.Fprintln(stdout, "  私有面门禁: 通过")
	} else {
		fmt.Fprintf(stderr, "  ❌ 公开树出现私有面路径 / 门取不到结论（rc=%d）——已中止导出（一个字节都不推）\n", privRC)
		return exitFail
	}
	// gitleaks（若装了）。
	if glBin, lerr := exec.LookPath("gitleaks"); lerr == nil {
		fmt.Fprintln(stdout, "  → 追加 gitleaks 通用密钥扫描")
		var gl *exec.Cmd
		if ppExistsFile(filepath.Join(out, ".gitleaks.toml")) {
			gl = armChildGroup(exec.Command(glBin, "detect", "--no-git", "--redact", "-v", "-c", ".gitleaks.toml"))
		} else {
			gl = armChildGroup(exec.Command(glBin, "detect", "--no-git", "--redact", "-v"))
		}
		gl.Dir = out
		gl.Stdout = stdout
		gl.Stderr = stderr
		if rc := ppRunChild(gl); rc != 0 {
			fmt.Fprint(stderr, "❌ gitleaks 命中——中止发布\n")
			return exitFail
		}
	} else {
		fmt.Fprintln(stdout, "  （未安装 gitleaks——跳过通用密钥扫描；brew install gitleaks 可启用）")
	}

	if opts.DryRun {
		ppSay(stdout, "dry-run：完成导出/变换/扫描，停在 git 之前。目录："+out)
		return exitOK
	}

	// 4b 文档双语门禁（按名调用 `check_docs`）。
	if rc := ppGate(root, "check_docs", stdout, stderr, []string{out}); rc == 0 {
		fmt.Fprintln(stdout, "  文档门禁: 通过")
	} else {
		fmt.Fprintln(stderr, "  ❌ 文档门禁未通过——已中止导出")
		return exitFail
	}
	// 4d i18n 门禁（命令面执行体 `ci18Run` · 输出抑制）。
	if rc := ci18Run(root, io.Discard, io.Discard, nil); rc == 0 {
		fmt.Fprintln(stdout, "  i18n 门禁: 通过")
	} else {
		fmt.Fprintln(stderr, "  ❌ i18n 门禁未通过——已中止导出")
		ci18Run(root, stderr, stderr, nil)
		return exitFail
	}
	// 4c 版本号一致（stdout 抑制；fail 时 stdout→stderr 重跑，照旧壳）。
	if rc := ppGate(root, "check_version", io.Discard, stderr, []string{root}); rc == 0 {
		fmt.Fprintln(stdout, "  版本号门禁: 通过")
	} else {
		fmt.Fprintln(stderr, "  ❌ 版本号不一致——已中止导出")
		ppGate(root, "check_version", stderr, stderr, []string{root})
		return exitFail
	}
	// 4e 兼容清单门禁（stdout 抑制；fail 时 stdout→stderr 重跑，照旧壳）。
	if rc := ppGate(root, "check-compat-manifest", io.Discard, stderr, []string{root}); rc == 0 {
		fmt.Fprintln(stdout, "  兼容清单门禁: 通过")
	} else {
		fmt.Fprintln(stderr, "  ❌ 跨版本状态兼容清单与代码不一致——已中止导出")
		ppGate(root, "check-compat-manifest", stderr, stderr, []string{root})
		return exitFail
	}

	// ── 5/6 生成单个压平提交（SQUASH）──
	ppSay(stdout, "5/6 生成单个压平提交（SQUASH）")
	_ = os.RemoveAll(filepath.Join(out, ".git"))
	if rc := ppGitRun(out, stdout, stderr, nil, "init", "-q", "-b", "main"); rc != 0 {
		return exitFail
	}
	if rc := ppGitRun(out, stdout, stderr, nil, "add", "-A"); rc != 0 {
		return exitFail
	}
	commitMsg := "Zerg Swarm (虫族) — 公开快照\n\n源码快照，非开发历史。完整开发史保留在私有权威仓。\n\n" +
		"GitOrigin-RevId: " + privHead + "\nLicense: Apache-2.0 (see LICENSE)\nThird-party: see THIRD_PARTY_LICENSES.md"
	env := []string{
		"GIT_AUTHOR_NAME=" + authorName, "GIT_AUTHOR_EMAIL=" + authorEmail,
		"GIT_COMMITTER_NAME=" + authorName, "GIT_COMMITTER_EMAIL=" + authorEmail,
	}
	if rc := ppGitRun(out, stdout, stderr, env, "commit", "-q", "-m", commitMsg); rc != 0 {
		return exitFail
	}
	if _, log := ppGit(out, "log", "--oneline", "-1"); true {
		fmt.Fprintf(stdout, "提交: %s\n", log)
	}
	nf := 0
	if _, o := ppGit(out, "ls-files"); true {
		if o != "" {
			nf = len(strings.Split(o, "\n"))
		}
	}
	duSize := ppDuSh(out)
	fmt.Fprintf(stdout, "文件: %d 个 / 体积: %s\n", nf, duSize)
	if _, who := ppGit(out, "log", "-1", "--format=%an <%ae>"); true {
		fmt.Fprintf(stdout, "作者: %s\n", who)
	}

	// ── 6/6 推送（--push 才走 · 默认只准备 · 不出网）──
	if opts.Push {
		ppSay(stdout, "6/6 推送到公开仓")
		if opts.Remote == "" {
			fmt.Fprintln(stderr, "需要 --remote 或 ZERG_PUBLIC_REMOTE")
			return exitUsage
		}
		if rc := ppGitRun(out, io.Discard, io.Discard, nil, "remote", "add", "origin", opts.Remote); rc != 0 {
			_ = ppGitRun(out, io.Discard, io.Discard, nil, "remote", "set-url", "origin", opts.Remote)
		}
		if rc := ppGitRun(out, stdout, stderr, nil, "push", "-f", "origin", "main"); rc != 0 {
			return exitFail
		}
		fmt.Fprintf(stdout, "已推送: %s\n", opts.Remote)
	} else {
		ppSay(stdout, "6/6 未推送（准备就绪）")
		fmt.Fprintf(stdout, "推送命令：  cd %s && git remote add origin <公开仓> && git push -f origin main\n", out)
		fmt.Fprintln(stdout, "或重跑：    scripts/build/publish-public.sh --push --remote <公开仓>")
	}
	return exitOK
}

// ppEnvOr —— 读环境变量，空则取默认（照旧壳 `:=`）。
func ppEnvOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// ppFirst12 —— 照 bash `${VAR:0:12}`（前 12 字节）。
func ppFirst12(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

// ppCountMatching —— 复刻 `find <root> -name '<suffix>' | wc -l`（递归）。
func ppCountMatching(root, suffix string) int {
	n := 0
	_ = filepath.Walk(root, func(_ string, fi os.FileInfo, err error) error {
		if err == nil && !fi.IsDir() && strings.HasSuffix(fi.Name(), suffix) {
			n++
		}
		return nil
	})
	return n
}

// ppDuSh —— 复刻 `du -sh . | cut -f1`（在 dir 里跑 · 取第一列）。
func ppDuSh(dir string) string {
	var out strings.Builder
	cmd := armChildGroup(exec.Command("du", "-sh", "."))
	cmd.Dir = dir
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	if ppRunChild(cmd) != 0 {
		return ""
	}
	f := strings.Fields(out.String())
	if len(f) > 0 {
		return f[0]
	}
	return ""
}
