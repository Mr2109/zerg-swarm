// check_public_tree_hazards.go —— 判据件
// `scripts/gates/check-public-tree-hazards.py`（414 行 · sha16 db8aea7683337e4d）的 **Go 真搬**（M2 批 · 甲案口径 · 公开面）。
//
// 本件做什么（= py `__main__` 内联主逻辑）：
//
//	发布期门禁：判「即将发布的产物树」里有无危险路径族与令牌形态。
//	退码：0 = 干净；1 = 有命中（不得发布）；2 = 拒绝出结论（自测未过 · 或目标不是公开产物树）。
//	两档：发布档（默认 · 目标须先过 tree_class 前置判据）· 审计档（`--all` 扫全史对象，预期为红）。
//	前置判据 tree_class 为纯函数（单提交 + 快照提交信息），判不出即 rc=2（缺语料铁律）。
//
// 子进程面（一律现调 git，与 py 同名同参）：
//
//	git -C <dir> log -1 --format=%s%x00%b                                  （probe_product_tree）
//	git -C <dir> rev-list --count HEAD                                      （probe_product_tree）
//	git -C <dir> -c core.quotepath=false ls-tree -r <commit> --name-only   （all_paths）
//	git -C <dir> -c core.quotepath=false rev-list --objects --all          （scan_paths · 审计档）
//	git -C <dir> rev-list main                                              （scan_forms · 审计档）
//	git -C <dir> grep -I -E -l -e <pattern> <commit> -- .                   （scan_forms）
//	（自检面另起仓外临时 git 仓：init/add/commit + ls-tree）
//
// 入口签名：
//
//	func checkPublicTreeHazardsGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码 —— 0/1/2（与 py 逐字对齐）
//	  · 输出 —— stdout/stderr 与 py 逐字节对齐（未覆盖面见下）
//
// 输出序的决定者（★ 逐点交代）：
//
//	· scan_paths 发布档：外层 `git ls-tree -r --name-only` 输出序（git 内部按树对象序），
//	  内层 PATH_RES 声明序（一条路径命中多模式即多行，**不 break**）。
//	· scan_paths 审计档：外层 PATH_RES 声明序，内层 `sorted(set(paths))`（码点序 ⟷ Go `sort.Strings`）。
//	· scan_forms 发布档：外层 FORM_RES 声明序，内层 `git grep` 输出行序。
//	· scan_forms 审计档：外层 `rev-list main` 提交序（新→旧），内层去重 `seen` 的首次遇到序。
//	· 明细输出仅取前 15 项（py `ph[:15]`/`fh[:15]`）。
//
// ── 标准声明 (a)：默认根定位差异 ──────────────────────────────────────────────
// py 的目标目录是**位置参数**（argv[0]，缺省 `.`），与 `__file__` 无关 ⇒ 本件同构。二者唯一语境差：
// 缺省 `.` 都是**进程 cwd**。⇒ 对拍一律显式传目标目录。外部真源（产物树）由
// `scripts/build/publish-preflight.sh:85` 以 `$D` 传入 —— 真身路径不落在本仓，本件走同名位置参数旋钮，
// ✗ 不硬编码任何仓外绝对路径。
//
// ── 标准声明 (b)：自指检查 ────────────────────────────────────────────────────
// 本件落在 `core/` 域内，故逐条核对：
//
//	① 本门的扫描面 = **目标 git 树的路径/内容**（由 argv 传入），不是 Go 模块目录 ⇒ 本件只在目标树
//	   含它时才被扫。落件为**未跟踪新件**（不在任何 git 树/对象里）⇒ 对本门读数贡献 0。受控对照：
//	   造一棵含本件的「产物树」影子仓，本门命中数 = 0（件名与正文均不触发任何族）。
//	② `check-hardcoded-private-paths`（扫 `core/`）：本件零私有绝对路径字面量（路径一律由 argv 传出）
//	   ⇒ 遗留悬空字面量 0。
//	③ `check-error-kinds`（扫 `core/cmd/zerg/*.go`）：本件不打印任何闭集 kind 字面量 ⇒ R3 命中数不变。
//	④ 若干门按「连续字面 `--self-test`」认门 ⇒ 本件是旗标所需连续字面（与同批已落件同为既定面）。
//	⑤ 本件正文**不含**任何可命中的令牌形态或完整危险路径原串：自测串一律**拼接构造**（与 py 同法），
//	   正则模式串本身不自匹配（`-` 后接 `[`、`Bearer ` 后接 `[` 均不在字符类内）。
//
// ── 未覆盖面（✗ 不声称逐字对齐）──────────────────────────────────────────────
//  1. `-h/--help`：py 无 argparse（纯位置参数 + 手判旗标）⇒ 无帮助正文面。
//  2. 位置参数个数 > 2：py 只读 argv[0]/argv[1]，多余项**静默忽略**；本件同。
//  3. 最外层无兜底（C 类真口子）：py `__main__` 裸跑，未捕异常 = 回溯 + rc=1。git 缺席时
//     `probe_product_tree` 首调 py 捕 OSError → 「调不起 git」文案；**第二次调用（rev-list --count）
//     py 不捕** ⇒ 该处为声明口径差：本件对第二次调用同样回落声明文案（非 2 档逐字沿用 py 其余部分）。
//  4. `str.isdigit()` 的 **Unicode 数字**（非 ASCII 档）：本件只认 ASCII ⇒ 该档未覆盖。
//  5. `errors="replace"` 的非 UTF-8 替换粒度（py 按非法子序列 ⟷ Go `ToValidUTF8` 按连续非法字节）：
//     真仓路径/内容多为合法 UTF-8 ⇒ 未触发，列此。
//  6. py 非多行 `$` 匹配「串尾换行之前」⟷ Go RE2 `$` 只匹配串尾：被匹配串是 git 输出的单行路径，
//     不含尾换行 ⇒ 未触发。
//  7. `%r`（Python repr）面：本件自写 `cphPyReprStr`/`cphPyReprStrList`/`cphPyReprPairList` 逐字复刻
//     本件用到的 repr 子集；对未出现的转义形态（孤立代理码点等）不声称对齐。
//  8. 自检面 fixture 用 `os.MkdirTemp`（Go 侧 `defer os.RemoveAll` 回收）—— 与 py
//     `mkdtemp`+`finally: rmtree` 两态均回收，回收差异不可见。
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// ── 判据 A：危险路径（存在性；按仓库根相对路径匹配）─────────────────────────

type cphRule struct {
	rx  *regexp.Regexp
	why string
}

var cphDangerPath = []cphRule{
	{regexp.MustCompile(`(^|/)\.zerg/`), "运行期任务日志/状态目录"},
	{regexp.MustCompile(`\.jsonl$`), "事件/日志流文件"},
	{regexp.MustCompile(`\.db$`), "SQLite 数据库"},
	{regexp.MustCompile(`\.db-(wal|shm)$`), "SQLite WAL/SHM"},
	{regexp.MustCompile(`\.sqlite$`), "SQLite 数据库"},
	{regexp.MustCompile(`(^|/)logs/`), "日志目录"},
	{regexp.MustCompile(`\.log$`), "日志文件"},
	{regexp.MustCompile(`(^|/)\.env$`), "真配置文件（示例应叫 .env.example）"},
	{regexp.MustCompile(`\.env\.local$`), "本地覆盖配置"},
	{regexp.MustCompile(`(^|/)target/`), "构建产物目录"},
	{regexp.MustCompile(`(^|/)node_modules/`), "依赖目录"},
	{regexp.MustCompile(`\.DS_Store$`), "macOS 目录元数据"},
}

// ── 判据 B：令牌形态（内容）──────────────────────────────────────────────────

var cphTokenForm = []cphRule{
	{regexp.MustCompile(`x3gw-[A-Za-z0-9_.\-]{6,}`), "项目网关令牌形态"},
	{regexp.MustCompile(`Bearer [A-Za-z0-9_.\-]{20,}`), "bearer + 长值（真凭据）"},
}

// 拼接构造自测串，避免本文件出现完整可命中的形态（与 py `T`/`B` 同法）。
const (
	cphT = "x3gw-"
	cphB = "Bearer "
)

// ── 前置判据（缺口 Q-177）：传入目录是不是「公开产物树」──────────────────────
const (
	cphSnapshotSubject  = "Zerg Swarm (虫族) — 公开快照" // 压平提交首行（publish-public.sh）
	cphSnapshotBodyMark = "GitOrigin-RevId:"       // 压平提交正文（第二道独立信号）
)

func cphTreeClass(commitCount *int, subject, body string) (bool, string) {
	if commitCount == nil {
		return false, "读不到提交数（不是 git 仓 / 空仓？）"
	}
	if *commitCount != 1 {
		return false, fmt.Sprintf("提交数 %s ≠ 1（产物树由压平器收成单提交）", strconv.Itoa(*commitCount))
	}
	if !strings.HasPrefix(subject, cphSnapshotSubject) {
		return false, fmt.Sprintf("提交首行不是快照首行（现读 %s）", cphPyReprStr(subject))
	}
	if !strings.Contains(body, cphSnapshotBodyMark) {
		return false, fmt.Sprintf("提交正文缺 `%s`（第二道独立信号）", cphSnapshotBodyMark)
	}
	return true, "单提交 + 快照提交信息（压平产物树）"
}

// cphGitOut —— 一次 git 调用的读数（capture_output=True 语义，rc 不抛）。
type cphGitOut struct {
	stdout string
	rc     int
	spawn  error // 非 nil = 进程都起不来（py 的 OSError）
}

// cphGitCapture —— 跑 git 并接住 stdout（text=True · utf-8 · errors=replace）。
func cphGitCapture(args []string) cphGitOut {
	cmd := exec.Command(args[0], args[1:]...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	err := cmd.Run()
	o := cphGitOut{stdout: cphToValidUTF8(out.String())}
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			o.rc = ee.ExitCode()
		} else {
			o.spawn = err
		}
	}
	return o
}

// cphProbeProductTree —— 读目标仓实况交给 cphTreeClass；读不到 ⇒ (False, 说清为何判不了)。
func cphProbeProductTree(gitdir string) (bool, string) {
	log := cphGitCapture([]string{"git", "-C", gitdir, "log", "-1", "--format=%s%x00%b"})
	if log.spawn != nil {
		return false, "调不起 git（本机无 git 可执行或权限不足）"
	}
	if log.rc != 0 || !strings.Contains(log.stdout, "\x00") {
		return false, fmt.Sprintf("读不到 HEAD 提交信息（非 git 目录 / 空仓：`%s`）", gitdir)
	}
	trimmed := strings.TrimRight(log.stdout, "\n") // py rstrip("\n")
	parts := strings.SplitN(trimmed, "\x00", 2)
	subject, body := parts[0], parts[1]
	cntOut := cphGitCapture([]string{"git", "-C", gitdir, "rev-list", "--count", "HEAD"})
	cnt := cphPyStrip(cntOut.stdout)
	var cntPtr *int
	if cphAllASCIIDigits(cnt) {
		n, err := strconv.Atoi(cnt)
		if err == nil {
			cntPtr = &n
		}
	}
	return cphTreeClass(cntPtr, subject, body)
}

// ── 自测 ─────────────────────────────────────────────────────────────────────

type cphPathCase struct {
	text string
	want bool
	why  string
}

type cphTreeCase struct {
	cnt  *int
	subj string
	body string
	want bool
	why  string
}

func cphIntPtr(n int) *int { return &n }

func cphCasesPath() []cphPathCase {
	return []cphPathCase{
		{"core/.zerg/logs/2026-01-01T00-00-00/events.jsonl", true, "运行期日志（本次事故的形态）"},
		{"data/knowledge.db", true, "SQLite 库"},
		{"data/knowledge.db-wal", true, "WAL"},
		{"var/logs/app.log", true, "日志目录/文件"},
		{"docs/logs.md", false, "文件名含 logs，但不是日志目录"},
		{"docs/guide.md", false, "正常文档"},
		{".env", true, "真配置"},
		{".env.example", false, "示例配置（正当）"},
		{"src/main.go", false, "正常源码"},
		{"ui/target/debug/x", true, "构建产物"},
	}
}

func cphCasesForm() []cphPathCase {
	return []cphPathCase{
		{"Authorization: " + cphB + "a1b2c3d4e5f6g7h8i9j0k1l2", true, "真 bearer 值"},
		{"Authorization: " + cphB + "<token>", false, "占位写法（正当）"},
		{"令牌以 " + cphT + " 开头", false, "说明文字（无实际值）"},
		{"X-Auth-Token: " + cphT + "shared0123456789", true, "实际令牌形态"},
		{"curl -H 'X-Auth-Token: $TOKEN' http://x", false, "变量引用"},
	}
}

func cphCasesTree() []cphTreeCase {
	return []cphTreeCase{
		{cphIntPtr(1), cphSnapshotSubject, "源码快照，非开发历史。" + cphSnapshotBodyMark + " abc", true,
			"压平产物树（发布档正控）"},
		{cphIntPtr(1), "feat: 私有开发提交", cphSnapshotBodyMark + " abc", false,
			"单提交但不是快照 ⇒ 判不了（不给结论）"},
		{cphIntPtr(1), cphSnapshotSubject, "正文无第二道信号", false, "缺 GitOrigin-RevId ⇒ 判不了"},
		{cphIntPtr(1332), cphSnapshotSubject, cphSnapshotBodyMark + " abc", false, "私有权威仓（多提交）⇒ 判不了"},
		{nil, cphSnapshotSubject, cphSnapshotBodyMark + " abc", false, "读不到提交数 ⇒ 判不了"},
	}
}

func cphMark(ok bool) string {
	if ok {
		return "✓"
	}
	return "✗"
}

func cphTF(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

// cphSelftestCJK —— 自检格①（CJK 件名不许失明 · 缺口 GAP-20260927-216）。
// 返回（打印格数, 失败增量）。夹具建不起 ⇒ 红（缺语料不许静默绿）。
func cphSelftestCJK(stdout io.Writer) (extra, bad int) {
	fxRoot, err := os.MkdirTemp("", "hazards-selftest-cjk-")
	if err != nil {
		fmt.Fprintf(stdout, "   ✗ %-32s | 夹具建不起来（%s）⇒ 不许静默绿\n", "cjk-name-clean-from-list", err)
		return 1, 1
	}
	defer os.RemoveAll(fxRoot)

	fxRepo := cphPyJoin(fxRoot, "repo")
	cjkRel := cphPyJoin("子目录", "中文名件.md")
	gitEnv := cphGitEnv()

	if err := os.MkdirAll(cphPyJoin(fxRepo, "子目录"), 0o755); err != nil {
		fmt.Fprintf(stdout, "   ✗ %-32s | 夹具建不起来（%s）⇒ 不许静默绿\n", "cjk-name-clean-from-list", err)
		return 1, 1
	}
	if err := os.WriteFile(cphPyJoin(fxRepo, cjkRel), []byte("x\n"), 0o644); err != nil {
		fmt.Fprintf(stdout, "   ✗ %-32s | 夹具建不起来（%s）⇒ 不许静默绿\n", "cjk-name-clean-from-list", err)
		return 1, 1
	}
	if err := os.WriteFile(cphPyJoin(fxRepo, "docs-ascii.md"), []byte("x\n"), 0o644); err != nil {
		fmt.Fprintf(stdout, "   ✗ %-32s | 夹具建不起来（%s）⇒ 不许静默绿\n", "cjk-name-clean-from-list", err)
		return 1, 1
	}
	for _, args := range [][]string{
		{"git", "init", "-q", "-b", "main", fxRepo},
		{"git", "-C", fxRepo, "add", "-A"},
		{"git", "-C", fxRepo, "commit", "-q", "-m", "fixture"},
	} {
		if err := cphGitFixture(args, gitEnv); err != nil {
			fmt.Fprintf(stdout, "   ✗ %-32s | 夹具建不起来（%s）⇒ 不许静默绿\n", "cjk-name-clean-from-list", err)
			return 1, 1
		}
	}

	listed := cphAllPaths(fxRepo, "HEAD")
	naive := cphGitCapture([]string{"git", "-C", fxRepo, "-c", "core.quotepath=true",
		"ls-tree", "-r", "HEAD", "--name-only"}).stdout
	naiveEscaped := false
	for _, p := range strings.Split(naive, "\n") {
		if cphPyStrip(p) == "" {
			continue
		}
		if strings.Contains(p, `"`) || cphReOctal3.MatchString(p) {
			naiveEscaped = true
			break
		}
	}
	statOK := true
	if _, err := os.Stat(cphPyJoin(fxRepo, cjkRel)); err != nil {
		statOK = false
	}

	contains := func(sl []string, v string) bool {
		for _, s := range sl {
			if s == v {
				return true
			}
		}
		return false
	}
	anyQuote := false
	anyOctal := false
	allExpected := true
	for _, p := range listed {
		if strings.HasPrefix(p, `"`) || strings.HasSuffix(p, `"`) {
			anyQuote = true
		}
		if cphReOctal3.MatchString(p) {
			anyOctal = true
		}
		if p != "docs-ascii.md" && p != cjkRel {
			allExpected = false
		}
	}

	checks := []struct {
		ok  bool
		why string
	}{
		{contains(listed, cjkRel), "中文名件在清单里（原样命中）"},
		{!anyQuote, "清单条目首尾无双引号"},
		{!anyOctal, "清单条目无反斜杠八进制转义"},
		{statOK, "按清单里的名字能 os.stat 到该件"},
		{allExpected, fmt.Sprintf("夹具清单只含这两件（现读 %s）", cphPyReprStrList(listed))},
	}
	for _, c := range checks {
		if !c.ok {
			bad++
		}
		extra++
		fmt.Fprintf(stdout, "   %s %-32s | %s\n", cphMark(c.ok), "cjk-name-clean-from-list", c.why)
	}
	teeth := naiveEscaped && strings.Contains(naive, "docs-ascii.md")
	if !teeth {
		bad++
	}
	extra++
	fmt.Fprintf(stdout, "   %s %-32s | %s\n", cphMark(teeth), "cjk-name-quotepath-is-real",
		"默认 quotePath 下 git 确实转义该名（⇒ 丢 `-c` 必红）")
	return extra, bad
}

// cphSelftestScan —— 自检格②（真扫描面不许哑化 · 缺口 GAP-20260926-216）。
func cphSelftestScan(stdout io.Writer) (extra, bad int) {
	fxRoot, err := os.MkdirTemp("", "hazards-selftest-scan-")
	if err != nil {
		for _, lbl := range []string{"scan-paths-catches-hazard", "scan-forms-catches-hazard"} {
			fmt.Fprintf(stdout, "   ✗ %-32s | 夹具建不起来（%s）⇒ 不许静默绿\n", lbl, err)
		}
		return 2, 2
	}
	defer os.RemoveAll(fxRoot)

	fxRepo := cphPyJoin(fxRoot, "repo")
	dangerPath := ".zerg/logs/events.jsonl"
	dangerBody := "note: " + cphT + "shared0123456789\n"
	gitEnv := cphGitEnv()

	fail := func(e error) (int, int) {
		for _, lbl := range []string{"scan-paths-catches-hazard", "scan-forms-catches-hazard"} {
			fmt.Fprintf(stdout, "   ✗ %-32s | 夹具建不起来（%s）⇒ 不许静默绿\n", lbl, e)
		}
		return 2, 2
	}
	if e := os.MkdirAll(cphPyJoin(fxRepo, ".zerg", "logs"), 0o755); e != nil {
		return fail(e)
	}
	if e := os.WriteFile(cphPyJoin(fxRepo, dangerPath), []byte(dangerBody), 0o644); e != nil {
		return fail(e)
	}
	if e := os.WriteFile(cphPyJoin(fxRepo, "docs-clean.md"), []byte("正常文档\n"), 0o644); e != nil {
		return fail(e)
	}
	for _, args := range [][]string{
		{"git", "init", "-q", "-b", "main", fxRepo},
		{"git", "-C", fxRepo, "add", "-A"},
		{"git", "-C", fxRepo, "commit", "-q", "-m", "fixture"},
	} {
		if e := cphGitFixture(args, gitEnv); e != nil {
			return fail(e)
		}
	}

	ph := cphScanPaths(fxRepo, false)
	phNamed := false
	for _, h := range ph {
		if h[0] == dangerPath {
			phNamed = true
		}
	}
	checks3 := []struct {
		ok  bool
		why string
	}{
		{len(ph) >= 1, fmt.Sprintf("scan_paths 对含危险路径的树真判红（现读 %d 项）", len(ph))},
		{phNamed, fmt.Sprintf("命中项逐字点名 `%s`（现读 %s）", dangerPath, cphPyReprPairList(ph))},
	}
	for _, c := range checks3 {
		if !c.ok {
			bad++
		}
		extra++
		fmt.Fprintf(stdout, "   %s %-32s | %s\n", cphMark(c.ok), "scan-paths-catches-hazard", c.why)
	}

	fh := cphScanForms(fxRepo, "HEAD", false)
	fhNamed := false
	for _, h := range fh {
		if strings.Contains(h[0], dangerPath) {
			fhNamed = true
		}
	}
	checks4 := []struct {
		ok  bool
		why string
	}{
		{len(fh) >= 1, fmt.Sprintf("scan_forms 对含令牌形态的树真判红（现读 %d 项）", len(fh))},
		{fhNamed, fmt.Sprintf("命中项逐字点名 `%s`（现读 %s）", dangerPath, cphPyReprPairList(fh))},
	}
	for _, c := range checks4 {
		if !c.ok {
			bad++
		}
		extra++
		fmt.Fprintf(stdout, "   %s %-32s | %s\n", cphMark(c.ok), "scan-forms-catches-hazard", c.why)
	}
	return extra, bad
}

// cphSelftest —— 与 py `selftest()` 逐字对齐。
func cphSelftest(stdout io.Writer) bool {
	bad := 0

	for _, c := range cphCasesPath() {
		got := false
		for _, rx := range cphDangerPath {
			if rx.rx.MatchString(c.text) {
				got = true
				break
			}
		}
		ok := got == c.want
		if !ok {
			bad++
		}
		fmt.Fprintf(stdout, "   %s want=%-5s got=%-5s | %s\n", cphMark(ok), cphTF(c.want), cphTF(got), c.why)
	}
	for _, c := range cphCasesForm() {
		got := false
		for _, rx := range cphTokenForm {
			if rx.rx.MatchString(c.text) {
				got = true
				break
			}
		}
		ok := got == c.want
		if !ok {
			bad++
		}
		fmt.Fprintf(stdout, "   %s want=%-5s got=%-5s | %s\n", cphMark(ok), cphTF(c.want), cphTF(got), c.why)
	}
	for _, c := range cphCasesTree() {
		got, _ := cphTreeClass(c.cnt, c.subj, c.body)
		ok := got == c.want
		if !ok {
			bad++
		}
		fmt.Fprintf(stdout, "   %s want=%-5s got=%-5s | %s\n", cphMark(ok), cphTF(c.want), cphTF(got), c.why)
	}

	extra := 0
	e1, b1 := cphSelftestCJK(stdout)
	extra += e1
	bad += b1
	e2, b2 := cphSelftestScan(stdout)
	extra += e2
	bad += b2

	total := len(cphCasesPath()) + len(cphCasesForm()) + len(cphCasesTree()) + extra
	verdict := "全过"
	if bad != 0 {
		verdict = fmt.Sprintf("%d 条失败", bad)
	}
	fmt.Fprintf(stdout, "   自测: %s（%d 格）\n", verdict, total)
	return bad == 0
}

// ── 扫描面 ───────────────────────────────────────────────────────────────────

// cphAllPaths —— 返回该提交里仓库根相对路径集合（ls-tree 原序）。
func cphAllPaths(gitdir, commit string) []string {
	out := cphGitCapture([]string{"git", "-C", gitdir, "-c", "core.quotepath=false",
		"ls-tree", "-r", commit, "--name-only"}).stdout
	res := []string{}
	for _, p := range strings.Split(out, "\n") {
		if cphPyStrip(p) != "" {
			res = append(res, p)
		}
	}
	return res
}

// cphScanPaths —— 危险路径判据。
func cphScanPaths(gitdir string, commitsAll bool) [][2]string {
	hits := [][2]string{}
	if commitsAll {
		out := cphGitCapture([]string{"git", "-C", gitdir, "-c", "core.quotepath=false",
			"rev-list", "--objects", "--all"}).stdout
		set := map[string]bool{}
		for _, ln := range strings.Split(out, "\n") {
			if strings.Contains(ln, " ") {
				set[strings.SplitN(ln, " ", 2)[1]] = true
			}
		}
		paths := make([]string, 0, len(set))
		for p := range set {
			paths = append(paths, p)
		}
		sort.Strings(paths)
		for _, rx := range cphDangerPath {
			for _, p := range paths {
				if rx.rx.MatchString(p) {
					hits = append(hits, [2]string{p, rx.why})
				}
			}
		}
	} else {
		for _, p := range cphAllPaths(gitdir, "HEAD") {
			for _, rx := range cphDangerPath {
				if rx.rx.MatchString(p) {
					hits = append(hits, [2]string{p, rx.why})
				}
			}
		}
	}
	return hits
}

// cphScanForms —— 令牌形态判据。
func cphScanForms(gitdir, commit string, commitsAll bool) [][2]string {
	hits := [][2]string{}
	if commitsAll {
		out := cphGitCapture([]string{"git", "-C", gitdir, "rev-list", "main"}).stdout
		commits := strings.Fields(out)
		pat := cphTokenForm[0].rx.String() + "|" + cphTokenForm[1].rx.String()
		seen := map[string]bool{}
		for _, c := range commits {
			o := cphGitCapture([]string{"git", "-C", gitdir, "grep", "-I", "-E", "-l",
				"-e", pat, c, "--", "."}).stdout
			for _, ln := range strings.Split(o, "\n") {
				if cphPyStrip(ln) == "" {
					continue
				}
				key := cphSplitColonLast(ln)
				if !seen[key] {
					seen[key] = true
					hits = append(hits, [2]string{key, fmt.Sprintf("令牌形态（提交 %s）", cphPySlice(c, 0, 8))})
				}
			}
		}
	} else {
		for _, rx := range cphTokenForm {
			o := cphGitCapture([]string{"git", "-C", gitdir, "grep", "-I", "-E", "-l",
				"-e", rx.rx.String(), commit, "--", "."}).stdout
			for _, ln := range strings.Split(o, "\n") {
				if cphPyStrip(ln) != "" {
					hits = append(hits, [2]string{cphSplitColonLast(ln), rx.why})
				}
			}
		}
	}
	return hits
}

// ── JSON 拒收（缺口 Q-176）：本门**无机器读面** ⇒ 给了 `--json` 不许沉默 ───────
var cphJSONFields = []string{}

func cphRefuseJSON(argv []string) *int {
	hit := []string{}
	for _, x := range argv {
		if x == "--json" || strings.HasPrefix(x, "--json=") {
			hit = append(hit, x)
		}
	}
	if len(hit) == 0 {
		return nil
	}
	quoted := make([]string, len(hit))
	for i, h := range hit {
		quoted[i] = "`" + h + "`"
	}
	rc := 2
	return &rc
}

// cphJSONRefusalText —— 拒收文案（唯一真源）。
func cphJSONRefusalText(stderr io.Writer, argv []string) {
	hit := []string{}
	for _, x := range argv {
		if x == "--json" || strings.HasPrefix(x, "--json=") {
			hit = append(hit, "`"+x+"`")
		}
	}
	fmt.Fprintf(stderr, "⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）\n", strings.Join(hit, " · "))
	fields := "无（本门只出人读面）"
	if len(cphJSONFields) > 0 {
		qs := make([]string, len(cphJSONFields))
		for i, f := range cphJSONFields {
			qs[i] = "`" + f + "`"
		}
		fields = strings.Join(qs, " · ")
	}
	fmt.Fprintf(stderr, "   字段表：%s\n", fields)
	fmt.Fprint(stderr, "⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形\n")
}

// ── 入口 ─────────────────────────────────────────────────────────────────────

func checkPublicTreeHazardsGateMain(stdout, stderr io.Writer, argv []string) int {
	// ① Q-176：给了 `--json`（本门无机器读面）⇒ 逐字说明后 rc=2（先于一切）。
	if rc := cphRefuseJSON(argv); rc != nil {
		cphJSONRefusalText(stderr, argv)
		return *rc
	}

	// ② `--self-test` / `--selftest`：给旗标 ⇒ 只跑自检（退出码即自检结论）。
	filtered := []string{}
	for _, a := range argv {
		if a != "--self-test" && a != "--selftest" {
			filtered = append(filtered, a)
		}
	}
	if len(filtered) != len(argv) {
		ok := cphSelftest(stdout)
		verdict := "**不过** ✗"
		if ok {
			verdict = "全过 ✓"
		}
		fmt.Fprintf(stdout, "── check-public-tree-hazards.py --self-test（旧拼法 --selftest 同档）：%s\n", verdict)
		if ok {
			return 0
		}
		return 2
	}

	// ③ 不给旗标 ⇒ 自检不过即拒绝出结论。
	if !cphSelftest(stdout) {
		fmt.Fprintln(stderr, "自测未过 ⇒ 脚本本身有问题，拒绝出结论")
		return 2
	}
	target := "."
	if len(filtered) > 0 {
		target = filtered[0]
	}
	mode := "HEAD"
	if len(filtered) > 1 {
		mode = filtered[1]
	}
	commitsAll := mode == "--all"

	// ④ 档位（Q-177）：自述与退码必须同向。
	if commitsAll {
		fmt.Fprint(stdout, "档位: 审计档（`--all` 扫全史对象）—— 供人工审计，**预期为红**；rc=1 只说「审计有命中」，不作发布结论\n")
	} else {
		isProduct, why := cphProbeProductTree(target)
		if !isProduct {
			fmt.Fprintf(stderr, "⛔ 拒绝出结论（rc=2）：目标不是「公开产物树」—— %s\n", why)
			fmt.Fprintf(stderr, "   目标: `%s` · 发布档只判**即将发布的产物树**（判不了就不给结论）\n", target)
			fmt.Fprint(stderr, "   口径: 私有/开发树请用审计档 `--all`；「真有危险物」与「判错了树」不许同形\n")
			return 2
		}
		fmt.Fprintf(stdout, "档位: 发布档（`%s` mode=%s）—— %s\n", target, mode, why)
	}

	ph := cphScanPaths(target, commitsAll)
	formCommit := mode
	if commitsAll {
		formCommit = "main"
	}
	fh := cphScanForms(target, formCommit, commitsAll)
	total := len(ph) + len(fh)

	if total == 0 {
		fmt.Fprintf(stdout, "结果: 通过——未发现危险路径与令牌形态（扫 %s mode=%s）\n", target, mode)
		return 0
	}

	fmt.Fprintf(stdout, "结果: 不通过——危险路径 %d 项、令牌形态 %d 项（%s mode=%s）：\n",
		len(ph), len(fh), target, mode)
	for _, h := range cphFirst15(ph) {
		fmt.Fprintf(stdout, "   路径 %s   ← %s\n", h[0], h[1])
	}
	for _, h := range cphFirst15(fh) {
		fmt.Fprintf(stdout, "   形态 %s   ← %s\n", h[0], h[1])
	}
	return 1
}

// ── 私有助手 ─────────────────────────────────────────────────────────────────

func cphFirst15(h [][2]string) [][2]string {
	if len(h) > 15 {
		return h[:15]
	}
	return h
}

// cphGitFixture —— 自检面 fixture 的 git 调用（py `check=True` 语义；-q 保证静默）。
func cphGitFixture(args []string, env []string) error {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = env
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

// cphGitEnv —— `dict(os.environ, GIT_*=t)` 同构（先剔同名再追加，避免 getenv 取首个）。
func cphGitEnv() []string {
	drop := map[string]bool{
		"GIT_AUTHOR_NAME": true, "GIT_AUTHOR_EMAIL": true,
		"GIT_COMMITTER_NAME": true, "GIT_COMMITTER_EMAIL": true,
	}
	env := []string{}
	for _, kv := range os.Environ() {
		k := kv
		if i := strings.IndexByte(kv, '='); i >= 0 {
			k = kv[:i]
		}
		if drop[k] {
			continue
		}
		env = append(env, kv)
	}
	env = append(env,
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@e",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@e")
	return env
}

var cphReOctal3 = regexp.MustCompile(`\\[0-7]{3}`)

// cphToValidUTF8 —— py `errors="replace"` 的近似（按连续非法字节序列替换一个 U+FFFD）。
func cphToValidUTF8(s string) string {
	return strings.ToValidUTF8(s, "\uFFFD")
}

// cphSplitColonLast —— py `ln.split(":", 1)[-1]`。
func cphSplitColonLast(ln string) string {
	if i := strings.Index(ln, ":"); i >= 0 {
		return ln[i+1:]
	}
	return ln
}

// cphPySlice —— py 码点切片 `s[a:b]`。
func cphPySlice(s string, a, b int) string {
	r := []rune(s)
	if a < 0 {
		a = 0
	}
	if b > len(r) {
		b = len(r)
	}
	if a > len(r) {
		a = len(r)
	}
	if a > b {
		return ""
	}
	return string(r[a:b])
}

// cphPyStrip —— py `str.strip()` 的 Unicode 空白（含 \x1c-\x1f；Go `unicode.IsSpace` 不剥）。
func cphPyStrip(s string) string {
	return strings.TrimFunc(s, cphIsPySpace)
}

func cphIsPySpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', '\u001c', '\u001d', '\u001e', '\u001f', '\u0085', '\u00a0':
		return true
	}
	return unicode.IsSpace(r)
}

// cphAllASCIIDigits —— py `str.isdigit()` 的 ASCII 子集（Unicode 数字档见未覆盖面）。
func cphAllASCIIDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// cphPyJoin —— 照 CPython posixpath.join 逐段规则直译（✗ 不用 filepath.Join）。
func cphPyJoin(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}
	path := parts[0]
	for _, b := range parts[1:] {
		if strings.HasPrefix(b, "/") {
			path = b
		} else if path == "" || strings.HasSuffix(path, "/") {
			path += b
		} else {
			path += "/" + b
		}
	}
	return path
}

// cphPyReprStr —— Python `repr(str)` 的本件用到的子集。
func cphPyReprStr(s string) string {
	hasSingle := strings.ContainsRune(s, '\'')
	hasDouble := strings.ContainsRune(s, '"')
	quote := byte('\'')
	if hasSingle && !hasDouble {
		quote = '"'
	}
	var b strings.Builder
	b.WriteByte(quote)
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r == rune(quote) {
				b.WriteByte('\\')
				b.WriteRune(r)
			} else if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&b, `\x%02x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte(quote)
	return b.String()
}

// cphPyReprStrList —— Python `repr(list[str])`。
func cphPyReprStrList(sl []string) string {
	parts := make([]string, len(sl))
	for i, s := range sl {
		parts[i] = cphPyReprStr(s)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// cphPyReprPairList —— Python `repr(list[tuple[str, str]])`。
func cphPyReprPairList(ps [][2]string) string {
	parts := make([]string, len(ps))
	for i, p := range ps {
		parts[i] = "(" + cphPyReprStr(p[0]) + ", " + cphPyReprStr(p[1]) + ")"
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
