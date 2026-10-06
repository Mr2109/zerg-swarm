package main

// check_i18n_drift.go —— `scripts/gates/check-i18n-drift.sh`（376 行 · bash）的 Go 真搬（M2 · 入口 `checkI18nDriftGateMain`）。
//
// 本件形态（件类 = 判据）：纯静态只读 —— 扫 `docs/<lang>/**`（frontmatter 标量 · 注释标记行）
// ＋ 只读 git（`rev-parse` / `cat-file -e` / `diff --exit-code --numstat` / `log -n1 --format=%ct`）。
// 零联网 · 零真 TTY · 零计时读数 · 零随机（唯一 `mktemp` 在 `--self-test` 自建仓，两处都回收）。
// 「确定读面子进程」= 只读 git，输入可锁、输出确定（同一仓同一 HEAD ⇒ 逐字可对拍）⇒ 适格成立。
// 退码仅 0/1/2（2 = 阻塞/不给结论）。
//
// 三相（各相独立退码 —— 禁尾部软检查）：
//   ① exist   存在性对照：`docs/<src>/<path>` 与 `docs/<dst>/<path>` **两侧都要在** ⇒ 阻塞 rc=2
//   ② stale   过期（commit 级）：译页记录的 `source_commit` ≠ 源页当前状态 ⇒ 阻塞 rc=2（只读 git）
//   ③ comment 源文注释：译页必须有 HTML 注释块 ⇒ 先非阻塞 rc=1（告警）
//
// 逐字节对齐要点（照脚本逐条现读）：
//  ① 遍历序：脚本 `find <dir> -type f -name '*.md' ! -name README.md | sort` ⇒
//    本件保 readdir 后 `sort.Strings`（`LC_COLLATE=C` ⇒ sort = 字节序 = Go 升序）。
//    排除面：`-name '*.md'`（基名后缀）· `! -name README.md` · `! -path '*_drafts/*'`
//    （fnmatch 无 FNM_PATHNAME ⇒ `*` 跨 `/` ⇒ 等价 `strings.Contains(p, "_drafts/")`）。
//    `-type f` 不跟随符号链接 ⇒ 用 `fs.DirEntry.Type().IsRegular()`（Lstat 语义），✗ 不用 os.Stat。
//  ② 词切分：`for f in $(list_pages …)` 按 IFS（空格/制表/换行）切、丢空 ⇒ 逐 token 复刻。
//  ③ `fm_value`（awk 行式 frontmatter 标量）自写 `ci18nFmValue`：`[[:space:]]` = 空格/\t/换行/\v/\f/\r；
//    剥引号 = 删所有 `"`；`substr($0, length(k)+2)` = 0 基下标 len(k)+1。
//  ④ `marker_value` 自写 `ci18nMarkerValue`：grep ERE 首匹配 + 三段 sed（BRE 左最贪）逐字复刻。
//  ⑤ `tr_out_of_repo` 用 `cd dirname && pwd` 的**物理**真身 ⇒ 自写 `ci18nPwdPhysical`（EvalSymlinks）。
//  ⑥ `--json` 面：脚本 `exec 3>&1 1>&2` ⇒ 人面转 stderr、stdout **只**留一行 JSON。
//    本件：`human = stderr`（json 时），JSON 行写 stdout。
//  ⑦ `HEAD=${HEAD_SHA%${HEAD_SHA#???????}}` = 前 7 字符（不足 7 则空）。
//
// 两条标准声明行：
//
//	(a) **默认根定位差异**：脚本 `ROOT` 缺省 `docs`（**相对 cwd**，再 `cd $(git rev-parse --show-toplevel)`）；
//	    本件无 `__file__` 语境，同从 cwd 经 `git rev-parse --show-toplevel` 上溯、再 `os.Chdir` 到仓根
//	    ⇒ 与脚本「cd 仓根 + 相对 docs」逐字同构（本件根推导不依赖脚本所在目录）。
//	(b) **自指检查**：本件扫描面 = `docs/<lang>/**`（默认 `docs/zh` / `docs/en`），**零 `.go` 件**
//	    ⇒ 本件（落 `core/cmd/zerg/`）**不在自身扫描面内**。量化：扫描对象仅 `.md` 正文，与 Go 源无关；
//	    全件退码仅 0/1/2、零私有绝对路径字面量、不写 `setErr(` 连续字面量。
//
// 未覆盖面（照实逐条，见回执）：① `--phase`/`--root`/`--langs` 缺值时 bash `shift 2` 越界会在
// stderr 多印一行 `shift: shift count out of range`（本件不印，故该极窄格不逐字）；② `--self-test`
// 第⑥格：脚本以 `bash "$SELF_ABS" --json` **起子进程**复跑自己，本件**进程内**复跑（输人面逐字同、
// 机制不同 —— 属「第二类·运行时可表达性差」，仅自检档豁免）；③ 非 UTF-8 输入件 / 读失败（`grep -c` 报错）
// 的 stderr 正文；④ 脚本 `-h` 只印前 40 行 `^#`（本件钉死该 40 行；脚本后续增删注释会漂）。
//
// 本单边界（照实）：**只落码 · 不接线** —— 不在 `main.go` 注册命令、不删件原件；入口
// `checkI18nDriftGateMain` 目前**无调用点**（M2 刻意状态）。

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ci18nHelp —— `-h|--help` 正文 = 脚本 `grep '^#' "$0" | head -40` 的逐字产物（脚本前 40 行 `^#` 行）。
const ci18nHelp = "#!/usr/bin/env bash\n# =============================================================================\n# check-i18n-drift.sh —— 双语漂移三件套（**命令形态** · 可原地复跑）\n# -----------------------------------------------------------------------------\n# 规格来源：Zerg-内部文档/01-设计/设计-文档体系-v1.0.md §0.6 P6 ⑤（守漂移三件套）\n#           依据调研：Zerg-内部文档/调研/调研-文档分块落地-检索与双语.md §2.4\n#\n# 三相（各相独立退码 —— 禁尾部软检查，设计稿 §6.2 血坑 1）：\n#   ① exist   存在性对照：docs/zh/<path> 与 docs/en/<path> **两侧都要在**\n#              对标 K8s scripts/linkchecker.py 头注释（本地化页要查「英文版在不在 + 中文版在不在」）\n#              **阻塞**（rc=2）\n#   ② stale   过期检测（**commit 级**）：译页记录的 `source_commit` ≠ 源页当前状态 ⇒ 过期\n#              对标 MDN frontmatter `l10n.sourceCommit`（「the commit hash of the upstream commit\n#              the translation is synchronized with」+「enforced by a linter」）\n#              与 K8s scripts/lsync.sh（`git log -n 1 … -- <译页>` + `git diff --exit-code --numstat`）\n#              **阻塞**（rc=2）。只读 git（rev-parse/log/diff），不写 git。\n#              ★ 2026-09-19 开发文档分家批6：源页可**迁出仓**（工作树之外的同级目录 `Zerg-内部文档/`）——\n#              那种情况下本门的 commit 级判据**不适用**（`git diff … -- <仓外路径>` 实测 rc=128\n#              `fatal: … is outside repository`，照原样跑会被读成「过期」= 假红）。处置 = 目标**存在但\n#              在仓外** ⇒ 单列「源页在仓外 · 不判过期」逐条打印 + 计数（`tr_out_of_repo()`），\n#              既不静默放绿、也不冒充过期；仓内源页照原判据判。\n#   ③ comment 源文注释：译页必须有 HTML 注释块（段落级保留源文，§2.4③）\n#              **先非阻塞**（rc=1 告警）—— 与设计稿「③ 先非阻塞」一致\n#\n# `source_commit` 从哪读（两处，前者优先；两处都读是为了不碰 frontmatter schema）：\n#   a) 译页 frontmatter 的 `source_commit:`（若 schema 将来放行该字段，直接生效）\n#   b) 源文注释块里的 `source_commit: <sha>` 标记行（**当前形态**：\n#      docs/site/frontmatter-schema.json 是 `additionalProperties: false` ⇒ 未申报字段写进\n#      frontmatter 即红 ⇒ 在补进 schema 之前，标记行落在注释里）\n#\n# 用法：\n#   scripts/gates/check-i18n-drift.sh [--phase all|exist|stale|comment] [--root docs] [--langs zh,en]\n#                               [--self-test|--selftest] [--json]\n# 退码：0 = 三相全绿 · 1 = 仅告警相命中（comment）\n#       2 = **不给结论 / 阻塞**（有阻塞相命中（exist/stale）· 环境/输入异常（不在仓根 / 无 git /\n#           扫到 0 篇 / 参数错）—— 不静默空转）\n#       ★ 2026-09-20 承接项「术语表只有一份」第 4 条：本脚本原 `rc=3` 一律**统一成 `rc=2`** ——\n#         与仓内其余门脚本（check-doc-name/meta/freshness、check-glossary.py、precommit-gates.sh）\n#         的 0/1/2 三档同一口径；对外效果 = 挂在公开树（无 `docs/` 面 ⇒ 扫到 0 篇）时落\n#         **BLOCKED（不给结论）**，而不是被 precommit-gates.sh 的 `tri-report` 档读成 FAIL。\n"

// ci18nArgs —— 已解析命令行（脚本 47-51 的缺省值）。
type ci18nArgs struct {
	phase    string
	root     string
	langs    string
	selftest bool
	json     bool
}

// ci18nArgOr —— 复刻 bash `${2:-default}`（**空串也回落缺省**）。
func ci18nArgOr(argv []string, i int, def string) string {
	if i < len(argv) && argv[i] != "" {
		return argv[i]
	}
	return def
}

// ci18nParse —— 逐字复刻脚本 53-65 行的 `case`（无 argparse / 无前缀缩写 / `--k=v` 不认）。
// 返回 (args, rc, done)：done=true ⇒ 已处理（帮助或错误），调用方直接回 rc。
func ci18nParse(argv []string, stdout, stderr io.Writer) (ci18nArgs, int, bool) {
	a := ci18nArgs{phase: "all", root: "docs", langs: "zh,en"}
	i := 0
	for i < len(argv) {
		switch argv[i] {
		case "--phase":
			a.phase = ci18nArgOr(argv, i+1, "all")
			i += 2
		case "--root":
			a.root = ci18nArgOr(argv, i+1, "docs")
			i += 2
		case "--langs":
			a.langs = ci18nArgOr(argv, i+1, "zh,en")
			i += 2
		case "--self-test", "--selftest":
			a.selftest = true
			i++
		case "--json":
			a.json = true
			i++
		case "-h", "--help":
			io.WriteString(stdout, ci18nHelp)
			return a, 0, true
		default:
			fmt.Fprintf(stderr, "✗ 未知参数：%s\n", argv[i])
			return a, 2, true
		}
	}
	return a, 0, false
}

// ci18nCutField —— 复刻 `printf '%s' s | cut -d, -f<n>`。
func ci18nCutField(s string, n int) string {
	if !strings.Contains(s, ",") {
		return s // cut：无分隔符的行原样输出
	}
	parts := strings.Split(s, ",")
	if n-1 >= 0 && n-1 < len(parts) {
		return parts[n-1]
	}
	return ""
}

// ---- git（只读）------------------------------------------------------------

// ci18nGit —— 跑只读 git，返回 (stdout, rc)。stderr 丢弃（照脚本 `2>/dev/null` / `2>&1`）。
func ci18nGit(args ...string) (string, int) {
	cmd := exec.Command("git", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	err := cmd.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return out.String(), ee.ExitCode()
		}
		return out.String(), -1
	}
	return out.String(), 0
}

func ci18nTrimNL(s string) string { return strings.TrimRight(s, "\n") }

// ci18nGitToplevel —— `git rev-parse --show-toplevel`；rc≠0 或空 ⇒ 不在仓内。
func ci18nGitToplevel() (string, bool) {
	out, rc := ci18nGit("rev-parse", "--show-toplevel")
	if rc != 0 {
		return "", false
	}
	v := ci18nTrimNL(out)
	if v == "" {
		return "", false
	}
	return v, true
}

// ci18nGitHEAD —— `git rev-parse HEAD`（`|| true` ⇒ 失败给空、rc 不算）。
func ci18nGitHEAD() (string, bool) {
	out, rc := ci18nGit("rev-parse", "HEAD")
	if rc != 0 {
		return "", false
	}
	return ci18nTrimNL(out), true
}

// ci18nGitLogCT —— `git log -n1 --pretty=format:%ct -- <path>`（空/失败 ⇒ 空串）。
func ci18nGitLogCT(path string) string {
	out, rc := ci18nGit("log", "-n", "1", "--pretty=format:%ct", "--", path)
	if rc != 0 {
		return ""
	}
	return ci18nTrimNL(out)
}

// ---- 文本原语 ---------------------------------------------------------------

// ci18nIsSpaceC —— awk/grep 的 `[[:space:]]`（C locale）= 空格/\t/\n/\v/\f/\r。
func ci18nIsSpaceC(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\v', '\f', '\r':
		return true
	}
	return false
}

// ci18nFileLines —— 复刻 awk/grep 的行读（按 `\n` 切；尾随 `\n` 不产生空行）。
func ci18nFileLines(path string) ([]string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	s := string(b)
	if s == "" {
		return nil, true
	}
	s = strings.TrimSuffix(s, "\n")
	return strings.Split(s, "\n"), true
}

// ci18nTrimC —— 剥 `[[:space:]]*`（前导/尾随）。
func ci18nStripLeadingSpace(s string) string {
	return strings.TrimLeftFunc(s, ci18nIsSpaceC)
}
func ci18nStripTrailingSpace(s string) string {
	return strings.TrimRightFunc(s, ci18nIsSpaceC)
}

// ci18nFmValue —— 复刻脚本 `fm_value`（awk 行式 frontmatter 标量键）。
func ci18nFmValue(path, key string) string {
	lines, ok := ci18nFileLines(path)
	if !ok {
		return ""
	}
	infm := false
	for i, line := range lines {
		nr := i + 1
		bar := ci18nIsOnlyDashes(line)
		if nr == 1 && !bar {
			return ""
		}
		if nr == 1 && bar {
			infm = true
			continue
		}
		if infm && bar {
			return ""
		}
		if infm {
			pre := key + ":"
			if strings.HasPrefix(line, pre) {
				v := line[len(pre):]
				v = ci18nStripLeadingSpace(v)
				v = ci18nStripTrailingSpace(v)
				v = strings.ReplaceAll(v, "\"", "")
				return v
			}
		}
	}
	return ""
}

// ci18nIsOnlyDashes —— `/^---[[:space:]]*$/`（`---` + 仅 C 空白）。
func ci18nIsOnlyDashes(s string) bool {
	if !strings.HasPrefix(s, "---") {
		return false
	}
	rest := s[3:]
	for _, r := range rest {
		if !ci18nIsSpaceC(r) {
			return false
		}
	}
	return true
}

// ci18nMarkerValue —— 复刻脚本 `marker_value`（grep -m1 -E + 三段 sed）。
func ci18nMarkerValue(path, key string) string {
	lines, ok := ci18nFileLines(path)
	if !ok {
		return ""
	}
	re := regexp.MustCompile(`^[ \t\v\f\r]*<!--?[ \t\v\f\r]*` + regexp.QuoteMeta(key) + `:`)
	line := ""
	found := false
	for _, l := range lines {
		if re.MatchString(l) {
			line = l
			found = true
			break
		}
	}
	if !found {
		return ""
	}
	// sed -e "s/.*KEY:[[:space:]]*//" ：贪婪删到「最后一个 KEY: 及其后随空白」
	line = ci18nSedStripToKey(line, key)
	// sed -e 's/-->.*$//' ：删除首个 `-->` 及其后全部
	if k := strings.Index(line, "-->"); k >= 0 {
		line = line[:k]
	}
	// sed -e 's/[[:space:]].*$//' ：删除首个 C 空白及其后全部
	if k := strings.IndexFunc(line, ci18nIsSpaceC); k >= 0 {
		line = line[:k]
	}
	return line
}

// ci18nSedStripToKey —— `s/.*KEY:[[:space:]]*//`（BRE 左最贪：取最后一个 `KEY:`）。
func ci18nSedStripToKey(line, key string) string {
	needle := key + ":"
	best := -1
	idx := 0
	for {
		k := strings.Index(line[idx:], needle)
		if k < 0 {
			break
		}
		best = idx + k
		idx = idx + k + len(needle)
	}
	if best < 0 {
		return line
	}
	v := line[best+len(needle):]
	return ci18nStripLeadingSpace(v)
}

// ci18nSourceCommitOf —— 先 frontmatter `source_commit`，再注释标记行。
func ci18nSourceCommitOf(path string) string {
	v := ci18nFmValue(path, "source_commit")
	if v != "" {
		return v
	}
	return ci18nMarkerValue(path, "source_commit")
}

// ci18nHasNonHex —— `case "$sc" in *[!0-9a-fA-F]*)` ：含任一非 hex 字符。
func ci18nHasNonHex(s string) bool {
	for _, r := range s {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') {
			continue
		}
		return true
	}
	return false
}

// ci18nGrepCountComment —— `grep -c '^[[:space:]]*<!--' <f>`（读失败 ⇒ 0）。
func ci18nGrepCountComment(path string) int {
	lines, ok := ci18nFileLines(path)
	if !ok {
		return 0
	}
	n := 0
	for _, l := range lines {
		t := ci18nStripLeadingSpace(l)
		if strings.HasPrefix(t, "<!--") {
			n++
		}
	}
	return n
}

// ---- 路径面 -----------------------------------------------------------------

func ci18nIsDir(p string) bool {
	fi, err := os.Stat(p) // 跟随符号链接（`[ -d ]`）
	return err == nil && fi.IsDir()
}

func ci18nIsFile(p string) bool {
	fi, err := os.Stat(p) // 跟随符号链接（`[ -f ]`）
	return err == nil && fi.Mode().IsRegular()
}

// ci18nBaseName —— 复刻 `basename`（剥尾斜杠；空 ⇒ `""`；`/` ⇒ `/`）。
func ci18nBaseName(p string) string {
	if p == "" {
		return ""
	}
	i := len(p)
	for i > 1 && p[i-1] == '/' {
		i--
	}
	s := p[:i]
	if s == "/" {
		return "/"
	}
	if j := strings.LastIndex(s, "/"); j >= 0 {
		return s[j+1:]
	}
	return s
}

// ci18nDirName —— 复刻 `dirname`（剥尾斜杠 → 去末段 → 空 ⇒ `.`）。
func ci18nDirName(p string) string {
	if p == "" {
		return "."
	}
	i := len(p)
	for i > 1 && p[i-1] == '/' {
		i--
	}
	s := p[:i]
	if s == "/" {
		return "/"
	}
	j := strings.LastIndex(s, "/")
	if j < 0 {
		return "."
	}
	k := j
	for k > 0 && s[k-1] == '/' {
		k--
	}
	if k == 0 {
		return "/"
	}
	return s[:k]
}

// ci18nPwdPhysical —— 复刻 `cd <d> && pwd`（物理真身；非目录/不存在 ⇒ false）。
func ci18nPwdPhysical(d string) (string, bool) {
	abs := d
	if !filepath.IsAbs(abs) {
		wd, err := os.Getwd()
		if err != nil {
			return "", false
		}
		abs = filepath.Join(wd, abs)
	}
	abs = filepath.Clean(abs)
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", false
	}
	fi, err := os.Stat(real)
	if err != nil || !fi.IsDir() {
		return "", false
	}
	return real, true
}

// ci18nTrOutOfRepo —— 复刻 `tr_out_of_repo`：目标（物理真身）落在 REPO_ROOT 之外 ⇒ true。
func ci18nTrOutOfRepo(p, repoRoot string) bool {
	if p == "" {
		return false
	}
	d, ok := ci18nPwdPhysical(ci18nDirName(p))
	if !ok || d == "" {
		return false
	}
	b := ci18nBaseName(p)
	joined := d + "/" + b
	if strings.HasPrefix(joined, repoRoot+"/") {
		return false // 仓内
	}
	return true // 仓外
}

// ci18nListPages —— 复刻 `list_pages`：`find <dir> -type f -name '*.md' ! -name README.md [!_drafts] | sort`。
func ci18nListPages(dir string, exclDrafts bool) []string {
	if !ci18nIsDir(dir) {
		return nil
	}
	var res []string
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // find 读不了就跳过该支
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() { // `-type f`：Lstat 语义，符号链接不算
			return nil
		}
		base := ci18nBaseName(p)
		if !strings.HasSuffix(base, ".md") {
			return nil
		}
		if base == "README.md" {
			return nil
		}
		if exclDrafts && strings.Contains(p, "_drafts/") {
			return nil
		}
		res = append(res, p)
		return nil
	})
	sort.Strings(res)
	return res
}

// ci18nListPagesFields —— `for f in $(list_pages …)`：按 IFS 切、丢空。
func ci18nListPagesFields(dir string, exclDrafts bool) []string {
	var out []string
	for _, p := range ci18nListPages(dir, exclDrafts) {
		out = append(out, strings.FieldsFunc(p, ci18nIsSpaceC)...)
	}
	return out
}

// ---- 三相 -------------------------------------------------------------------

type ci18nCtx struct {
	root, srcLang, dstLang string
	repoRoot, headSHA      string

	existScan, existMiss      int
	stalePages, staleHits     int
	commentPages, commentMiss int

	existRC, staleRC, commentRC int
}

// ci18nTierOf —— 复刻 `tier_of`：空转（scan=0 且 emptyBlocked）⇒ BLOCKED；miss>0 ⇒ FAIL；否则 PASS。
func ci18nTierOf(scan, miss int, emptyBlocked bool) string {
	if emptyBlocked && scan == 0 {
		return "BLOCKED"
	}
	if miss > 0 {
		return "FAIL"
	}
	return "PASS"
}

// ci18nHead7 —— `${HEAD_SHA%${HEAD_SHA#???????}}`：前 7 字符（不足 7 ⇒ 空）。
func ci18nHead7(s string) string {
	if len(s) >= 7 {
		return s[:7]
	}
	return ""
}

func ci18nPhaseExist(ctx *ci18nCtx, out io.Writer) int {
	rc := 0
	n := 0
	miss := 0
	srcDir := ctx.root + "/" + ctx.srcLang
	dstDir := ctx.root + "/" + ctx.dstLang
	for _, f := range ci18nListPagesFields(srcDir, true) {
		n++
		rel := strings.TrimPrefix(f, srcDir+"/")
		other := dstDir + "/" + rel
		if !ci18nIsFile(other) {
			fmt.Fprintf(out, "   ✗ %s  缺 %s 侧：%s\n", f, ctx.dstLang, other)
			miss++
		}
	}
	for _, f := range ci18nListPagesFields(dstDir, true) {
		n++
		rel := strings.TrimPrefix(f, dstDir+"/")
		other := srcDir + "/" + rel
		if !ci18nIsFile(other) {
			fmt.Fprintf(out, "   ✗ %s  缺 %s 侧：%s\n", f, ctx.srcLang, other)
			miss++
		}
	}
	fmt.Fprintf(out, "   ① 存在性对照：%s/%s 两侧共扫描 %d 个页面名，缺一侧 %d 个\n", ctx.srcLang, ctx.dstLang, n, miss)
	ctx.existScan = n
	ctx.existMiss = miss
	if n == 0 {
		fmt.Fprint(out, "   ⚠ 两侧合计 0 篇 —— 正式面还没建？⇒ 不给结论（rc=2，2026-09-20 由 rc=3 统一过来）\n")
		rc = 2
	}
	if miss != 0 {
		rc = 2
	}
	return rc
}

func ci18nPhaseStale(ctx *ci18nCtx, out io.Writer) int {
	rc := 0
	n := 0
	stale := 0
	nodate := 0
	outofrepo := 0
	for _, f := range ci18nListPagesFields(ctx.root+"/"+ctx.dstLang, false) {
		tr := ci18nFmValue(f, "translation_of")
		if tr == "" {
			continue
		}
		n++
		if !ci18nIsFile(tr) {
			fmt.Fprintf(out, "   ✗ %s  translation_of 指向的文件不存在：%s\n", f, tr)
			stale++
			continue
		}
		if ci18nTrOutOfRepo(tr, ctx.repoRoot) {
			fmt.Fprintf(out, "   ⚠ %s  源页已在**仓外**（%s）—— commit 级判据不适用，不判过期、不进退码（2026-09-19 开发文档分家）\n", f, tr)
			outofrepo++
			continue
		}
		sc := ci18nSourceCommitOf(f)
		if sc != "" {
			if ci18nHasNonHex(sc) {
				fmt.Fprintf(out, "   ⚠ %s  source_commit 形态不合法（不是 hex sha）：%s —— 不判过期\n", f, sc)
				nodate++
				continue
			}
			if _, grc := ci18nGit("cat-file", "-e", sc+"^{commit}"); grc != 0 {
				fmt.Fprintf(out, "   ⚠ %s  source_commit 指向的提交在本仓不存在：%s —— 不判过期\n", f, sc)
				nodate++
				continue
			}
			if _, grc := ci18nGit("diff", "--exit-code", "--numstat", sc+"...HEAD", "--", tr); grc != 0 {
				fmt.Fprintf(out, "   ✗ %s  过期：%s 在 %s..HEAD 之间改动过（source_commit=%s）\n", f, tr, sc, sc)
				stale++
				continue
			}
			if _, grc := ci18nGit("diff", "--exit-code", "--numstat", "HEAD", "--", tr); grc != 0 {
				fmt.Fprintf(out, "   ⚠ %s  源页有未提交改动（commit 级判据看不见）：%s\n", f, tr)
			}
		} else {
			nodate++
			srcLast := ci18nGitLogCT(tr)
			trLast := ci18nGitLogCT(f)
			if srcLast == "" || trLast == "" {
				fmt.Fprintf(out, "   ⚠ %s  二者之一无提交历史（新文件？）—— 不判过期，只记账\n", f)
				continue
			}
			sl, _ := strconv.Atoi(srcLast)
			tl, _ := strconv.Atoi(trLast)
			if sl > tl {
				fmt.Fprintf(out, "   ✗ %s  过期（lsync 形态）：源页最后一次提交晚于译页\n", f)
				stale++
			}
		}
	}
	fmt.Fprintf(out, "   ② 过期检测：%d 篇译页（%d 篇未记 source_commit ⇒ 退 lsync 形态；%d 篇源页在仓外 ⇒ 不判过期），过期 %d 篇\n", n, nodate, outofrepo, stale)
	ctx.stalePages = n
	ctx.staleHits = stale
	if stale != 0 {
		rc = 2
	}
	return rc
}

func ci18nPhaseComment(ctx *ci18nCtx, out io.Writer) int {
	rc := 0
	n := 0
	nocomment := 0
	for _, f := range ci18nListPagesFields(ctx.root+"/"+ctx.dstLang, false) {
		n++
		c := ci18nGrepCountComment(f)
		if c == 0 {
			fmt.Fprintf(out, "   ⚠ %s  无 HTML 注释块（未按 §2.4③ 段落级保留源文）\n", f)
			nocomment++
		}
	}
	fmt.Fprintf(out, "   ③ 源文注释：%d 篇译页，缺注释 %d 篇（先非阻塞）\n", n, nocomment)
	ctx.commentPages = n
	ctx.commentMiss = nocomment
	if nocomment != 0 {
		rc = 1
	}
	return rc
}

// ci18nRun —— 跑相 + 汇总（脚本 320-376）。
func ci18nRun(a ci18nArgs, srcLang, dstLang, repoRoot, headSHA string, stdout, stderr io.Writer) int {
	human := stdout
	if a.json {
		human = stderr
	}
	ctx := &ci18nCtx{
		root: a.root, srcLang: srcLang, dstLang: dstLang,
		repoRoot: repoRoot, headSHA: headSHA,
	}
	runAll := a.phase == "all"
	if runAll || a.phase == "exist" {
		fmt.Fprint(human, "── 相① 存在性对照（阻塞）\n")
		ctx.existRC = ci18nPhaseExist(ctx, human)
	}
	if runAll || a.phase == "stale" {
		fmt.Fprintf(human, "── 相② 过期检测 · commit 级（阻塞；只读 git；HEAD=%s）\n", ci18nHead7(headSHA))
		ctx.staleRC = ci18nPhaseStale(ctx, human)
	}
	if runAll || a.phase == "comment" {
		fmt.Fprint(human, "── 相③ 源文注释（先非阻塞）\n")
		ctx.commentRC = ci18nPhaseComment(ctx, human)
	}
	total := 0
	for _, r := range []int{ctx.existRC, ctx.staleRC, ctx.commentRC} {
		if r > total {
			total = r
		}
	}
	existTier := ci18nTierOf(ctx.existScan, ctx.existMiss, true)
	staleTier := ci18nTierOf(ctx.stalePages, ctx.staleHits, false)
	commentTier := ci18nTierOf(ctx.commentPages, ctx.commentMiss, false)
	if commentTier == "FAIL" {
		commentTier = "WARN"
	}
	drift := ctx.existMiss + ctx.staleHits
	overall := "PASS"
	twoTiers := existTier + "/" + staleTier
	if strings.Contains(twoTiers, "BLOCKED") {
		overall = "BLOCKED"
	} else if strings.Contains(twoTiers, "FAIL") {
		overall = "FAIL"
	}
	if a.json {
		fmt.Fprintf(stdout, "{\"schema\":\"zerg/i18n-drift/v1\",\"drift\":%d,\"verdict\":\"%s\",\"tiers\":{\"exist\":\"%s\",\"stale\":\"%s\",\"comment\":\"%s\"},\"counts\":{\"exist\":%d,\"stale\":%d,\"comment\":%d},\"rc\":%d}\n",
			drift, overall, existTier, staleTier, commentTier,
			ctx.existMiss, ctx.staleHits, ctx.commentMiss, total)
	}
	fmt.Fprint(human, "\n")
	fmt.Fprint(human, "── 汇总（机器读面 · 波10 序105 `W-22`）\n")
	fmt.Fprintf(human, "漂移条数：%d（= exist 未译 %d + stale 过期 %d）\n", drift, ctx.existMiss, ctx.staleHits)
	fmt.Fprintf(human, "判定档：%s（两格 = exist=%s · stale=%s）\n", overall, existTier, staleTier)
	fmt.Fprintf(human, "计数（三格分开）：exist 未译 %d · stale 过期 %d · comment 缺注释 %d（%s）\n", ctx.existMiss, ctx.staleHits, ctx.commentMiss, commentTier)
	fmt.Fprint(human, "\n")
	fmt.Fprintf(human, "退码：exist=%d stale=%d comment=%d ⇒ 总码=%d\n", ctx.existRC, ctx.staleRC, ctx.commentRC, total)
	return total
}

// ---- 自检 -------------------------------------------------------------------

var ci18nSelftestOut io.Writer

// ci18nGitIn —— 自检建仓用：在 dir 里跑 git（静默）。
func ci18nGitIn(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

// ci18nGitOutIn —— 自检建仓用：在 dir 里跑 git 取 stdout。
func ci18nGitOutIn(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return out.String(), err
	}
	return out.String(), nil
}

func ci18nWrite(path, body string) error {
	return os.WriteFile(path, []byte(body), 0o644)
}

// ci18nSelftestDrift —— 第⑥格：负控「过期 ≠ 未译」（自建 git 仓造 commit 级状态）。
func ci18nSelftestDrift() bool {
	d, err := os.MkdirTemp(os.TempDir(), "drift-negctl.")
	if err != nil {
		return false
	}
	defer os.RemoveAll(d)

	build := func() bool {
		if err := ci18nGitIn(d, "init", "-q", "."); err != nil {
			return false
		}
		if err := ci18nGitIn(d, "config", "user.email", "negctl@local"); err != nil {
			return false
		}
		if err := ci18nGitIn(d, "config", "user.name", "negctl"); err != nil {
			return false
		}
		if err := os.MkdirAll(filepath.Join(d, "docs", "zh"), 0o755); err != nil {
			return false
		}
		if err := os.MkdirAll(filepath.Join(d, "docs", "en"), 0o755); err != nil {
			return false
		}
		if err := ci18nWrite(filepath.Join(d, "docs", "zh", "s.md"), "---\ntitle: s\n---\nsrc v1\n"); err != nil {
			return false
		}
		if err := ci18nGitIn(d, "add", "-A"); err != nil {
			return false
		}
		if err := ci18nGitIn(d, "commit", "-qm", "src v1"); err != nil {
			return false
		}
		scOut, err := ci18nGitOutIn(d, "rev-parse", "HEAD")
		if err != nil {
			return false
		}
		sc := ci18nTrimNL(scOut)
		trBody := "---\ntranslation_of: docs/zh/s.md\nsource_commit: " + sc + "\n---\n<!-- 源文保留 -->\ntr\n"
		if err := ci18nWrite(filepath.Join(d, "docs", "en", "s.md"), trBody); err != nil {
			return false
		}
		if err := ci18nGitIn(d, "add", "-A"); err != nil {
			return false
		}
		if err := ci18nGitIn(d, "commit", "-qm", "tr"); err != nil {
			return false
		}
		if err := ci18nWrite(filepath.Join(d, "docs", "zh", "s.md"), "---\ntitle: s\n---\nsrc v2\n"); err != nil {
			return false
		}
		if err := ci18nGitIn(d, "add", "-A"); err != nil {
			return false
		}
		if err := ci18nGitIn(d, "commit", "-qm", "src v2"); err != nil {
			return false
		}
		return true
	}
	if !build() {
		return false
	}

	out, rcv := ci18nRunInDir(d, []string{"--json"})
	em, emOK := ci18nJSONInt(out, "exist")
	st, stOK := ci18nJSONInt(out, "stale")
	if emOK && stOK && em == 0 && st == 1 && rcv == 2 {
		fmt.Fprint(ci18nSelftestOut, "  ✓ 负控：过期件只落「stale 过期」（exist 未译 0 · stale 过期 1 · rc=2）\n")
		return true
	}
	emS, stS := "取不到", "取不到"
	if emOK {
		emS = strconv.Itoa(em)
	}
	if stOK {
		stS = strconv.Itoa(st)
	}
	fmt.Fprintf(ci18nSelftestOut, "  ✗ 负控：过期件的落格不对（exist 未译=%s · stale 过期=%s · rc=%d）\n", emS, stS, rcv)
	return false
}

// ci18nRunInDir —— 在 dir 里进程内复跑本判据（自检第⑥格用），返回 (stdout, rc)。
func ci18nRunInDir(dir string, argv []string) (string, int) {
	old, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(old) }()
	var out, errb bytes.Buffer
	rc := checkI18nDriftGateMain(&out, &errb, argv)
	return out.String(), rc
}

// ci18nJSONInt —— 从 JSON 行里取 `"<key>":<digits>`（对应 sed 提取 counts 面）。
func ci18nJSONInt(s, key string) (int, bool) {
	re := regexp.MustCompile(`"` + regexp.QuoteMeta(key) + `":([0-9]+)`)
	m := re.FindStringSubmatch(s)
	if m == nil {
		return 0, false
	}
	v, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, false
	}
	return v, true
}

// ci18nSelftest —— 复刻脚本 `selftest`（6 条）。
func ci18nSelftest(stdout io.Writer, repoRoot string) int {
	ci18nSelftestOut = stdout
	fmt.Fprint(stdout, "── check-i18n-drift.sh 自证（6 条 · 第 ⑥ 格建**自建** 临时仓造夹具；不碰本仓 git 写）\n")
	ok := 0
	tmp, err := os.MkdirTemp(os.TempDir(), "drift-selftest.")
	if err != nil {
		return 1
	}
	_ = ci18nWrite(filepath.Join(tmp, "a.md"), "---\ntranslation_of: docs/x.md\nsource_commit: abc123\n---\nbody\n")
	a := ci18nFmValue(filepath.Join(tmp, "a.md"), "translation_of")
	b := ci18nFmValue(filepath.Join(tmp, "a.md"), "source_commit")
	if a == "docs/x.md" && b == "abc123" {
		fmt.Fprint(stdout, "  ✓ frontmatter 标量读取\n")
		ok++
	} else {
		fmt.Fprintf(stdout, "  ✗ frontmatter 标量读取（%s / %s）\n", a, b)
	}
	_ = ci18nWrite(filepath.Join(tmp, "b.md"), "x\n<!-- source_commit: fd69646 -->\n")
	b = ci18nMarkerValue(filepath.Join(tmp, "b.md"), "source_commit")
	if b == "fd69646" {
		fmt.Fprint(stdout, "  ✓ 注释标记行读取\n")
		ok++
	} else {
		fmt.Fprintf(stdout, "  ✗ 注释标记行读取（%s）\n", b)
	}
	a = ci18nSourceCommitOf(filepath.Join(tmp, "b.md"))
	if a == "fd69646" {
		fmt.Fprint(stdout, "  ✓ 两处回退链（frontmatter → 标记行）\n")
		ok++
	} else {
		fmt.Fprintf(stdout, "  ✗ 两处回退链（%s）\n", a)
	}
	_ = ci18nWrite(filepath.Join(tmp, "c.md"), "no frontmatter here\n")
	a = ci18nFmValue(filepath.Join(tmp, "c.md"), "translation_of")
	if a == "" {
		fmt.Fprint(stdout, "  ✓ 无 frontmatter 不误判\n")
		ok++
	} else {
		fmt.Fprintf(stdout, "  ✗ 无 frontmatter 不误判（%s）\n", a)
	}
	_ = ci18nWrite(filepath.Join(tmp, "out.md"), "x\n")
	rcOut := 1
	if ci18nTrOutOfRepo(filepath.Join(tmp, "out.md"), repoRoot) {
		rcOut = 0
	}
	rcIn := 0
	if !ci18nTrOutOfRepo(repoRoot+"/AGENTS.md", repoRoot) { // 仓内 ⇒ 1
		rcIn = 1
	}
	if rcOut == 0 && rcIn == 1 {
		fmt.Fprint(stdout, "  ✓ 源页在仓外可识别（仓外 ⇒ 不判过期；仓内 ⇒ 照判）\n")
		ok++
	} else {
		fmt.Fprintf(stdout, "  ✗ 源页在仓外可识别（仓外 rc=%d 期望 0 / 仓内 rc=%d 期望 1）\n", rcOut, rcIn)
	}
	_ = os.RemoveAll(tmp)
	if ci18nSelftestDrift() {
		ok++
	}
	if ok == 6 {
		fmt.Fprint(stdout, "  自证：通过 ✓（6 条）\n")
		return 0
	}
	fmt.Fprintf(stdout, "  自证：**失败** ✗（%d/6）\n", ok)
	return 1
}

// checkI18nDriftGateMain —— 入口（= 脚本 `main` 流：解析 → langs 校验 → git 前置 → 自检/跑相）。
// argv **不含程序名**（inproc 口径）。返回进程退码 0/1/2。
func checkI18nDriftGateMain(stdout, stderr io.Writer, argv []string) int {
	a, rc, done := ci18nParse(argv, stdout, stderr)
	if done {
		return rc
	}
	srcLang := ci18nCutField(a.langs, 1)
	dstLang := ci18nCutField(a.langs, 2)
	if srcLang == "" || dstLang == "" {
		fmt.Fprint(stderr, "✗ --langs 需要两个语言码（如 zh,en）\n")
		return 2
	}
	repoRoot, ok := ci18nGitToplevel()
	if !ok {
		fmt.Fprint(stderr, "✗ 不在 git 仓内（⑵ 过期检测要读 commit 历史）⇒ 拒绝给结论\n")
		return 2
	}
	if err := os.Chdir(repoRoot); err != nil {
		return 2
	}
	headSHA, ok := ci18nGitHEAD()
	if !ok || headSHA == "" {
		fmt.Fprint(stderr, "✗ 拿不到 HEAD（空仓？）⇒ 拒绝给结论\n")
		return 2
	}
	if a.selftest {
		return ci18nSelftest(stdout, repoRoot)
	}
	return ci18nRun(a, srcLang, dstLang, repoRoot, headSHA, stdout, stderr)
}
