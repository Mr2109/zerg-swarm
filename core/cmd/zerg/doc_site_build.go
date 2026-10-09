// doc_site_build.go —— 文档站「导出 + 构建」的 **CLI 正门**（`zerg doc site build`）· 真搬（行为落 Go）。
//
// 依据：`~/.zerg/state/A档18件-收CLI设计稿-20261007.md` 批 2 · A14
//
//	（`docs/site/export-and-build.sh` · 308 行 · 现属面＝文档站面 · 归族 = 现役 `doc` 族扩 action）。
//
// 落法＝**真搬**（2026-10-08 · 本单）：把那个脚本的四段编排逐段落进本件 ——
//
//	① 参数面（`--strict` / `--serve` / `-h|--help` / 未知参数退 2）；
//	② 路径与环境文本（仓根 / python / venv / 构建目录 / 文档源 / 仓外取源三态）；
//	③ 导出树产出（nav 即白名单：解析 `mkdocs.yml` 的 nav、先仓内后仓外取源、生成占位首页、非 UTF-8 检查）；
//	④ 断言与计数（`config.lang == ['zh']` ＋ 索引含 U+200B）与退码（0 全绿 / 1 构建有失败项 / 2 不给结论）。
//
// 命令面**不再 exec 那个脚本**；两态（脚本在盘 ⟷ 移出仓）三面（rc/stdout/stderr）逐字同。
//
// ★ **保留为薄壳调用**（显式声明：不可在 Go 内等价复现的外部工具/系统命令 —— ✗ 不假装全搬）：
//
//	· `python -m venv` 建 venv ＋ `pip install -r tools/site-requirements.txt` 装 pin 依赖 —— 真装第三方包、联网；
//	· venv「实测版本」块（读 `importlib.metadata`）—— 查已装包元数据须那个解释器自身；
//	· `mkdocs build [--strict]` 与 `mkdocs serve` —— 站点工具链本体（第三方）。
//	  这四处仍以**子进程直调那些外部工具**（✗ 不 exec 被搬的那个 shell 脚本），逐字沿用原参数与重定向。
//
// ★ 出厂声明 (a) **默认根定位差异**：sh 从 `$0`（`SCRIPT_DIR/../..`）上溯取仓根；Go 无脚本路径语境 ⇒ 走
//
//	`repoRoot()`（`ZERG_REPO` 优先，否则锚 `core/internal/version/version.go`）。「被调脚本位置 ≠ 本命令
//	仓根」时两侧推导面不同 —— 本件不声称逐字对齐这一面。
//
// (b) **自指检查**：本件**不在**自身扫描面内（本件无自扫逻辑；正文不含会被门扫的私有绝对路径原串）。
//
// 未覆盖面（✗ 不声称逐字对齐 · 逐条声明）：
//
//	· `mkdocs.yml` 读不了 / YAML 解析失败 / `nav` 键缺失时 py 抛未捕异常（stderr = 解释器回溯 · rc=1）——
//	  本件按「读/解析失败」退 1（不复刻回溯正文）。
//	· 非 UTF-8 检查命中时 py 打 `UnicodeDecodeError` 的 `str()` —— 本件给出近似正文（实测真仓恒不触发）。
//	· 索引 JSON 解析失败时 py 抛未捕异常（rc=1）—— 本件退 1 并给近似正文。
//	· `os.walk` 的符号链接/权限语义、py 的 Unicode 空白 strip 面 —— 真仓不触发。
//	· 墙钟秒数取自 `date +%s` 的整秒差 —— 本件同样取整秒（值随机器/时刻漂，属非确定面）。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// docSiteBuildHelpLines —— `--help`（= `-h`）的正文 = 原脚本第 2–20 行（`sed -n '2,20p' "$0"` 的逐字内容）。
// 真搬后不再读脚本取它 ⇒ 内化为常量。★ 已声明漂移面：原脚本那 19 行若改，本常量须同批改。
var docSiteBuildHelpLines = []string{
	"# =============================================================================",
	"# 虫族文档站 · 导出 + 构建（W1 站点骨架 · §8.4④ 形态 B「独立导出树」）",
	"# -----------------------------------------------------------------------------",
	"# 它做什么（四步，全部**在 /tmp 里做**，不改仓、不 git、不发布）：",
	"#   1. 建/复用 venv（/tmp/zerg-site-venv），按 tools/site-requirements.txt 装 pin 版本",
	"#   2. 按 docs/site/mkdocs.yml 的 **nav 即白名单**，把正式面的 md **拷**进导出树",
	"#      （过程档案一条不拷；nav 未列的 md 也不会出现在导出树里）",
	"#   3. `mkdocs build [--strict]`",
	"#   4. 断言：index 语言 == zh、索引含 U+200B（jieba 生效过），并报页数/体积/退码",
	"#",
	"# 为什么是「导出树」而不是 exclude_docs：exclude_docs 把跨面链降成 INFO",
	"#   （不红，但产物里那条 <a href> 仍然写着 ⇒ 读者 404、门禁给绿）。导出树让跨面链",
	"#   **硬失败**。依据 docs/调研/调研-文档分块落地-分面与规范.md A2.1/A2.2。",
	"#",
	"# 用法：",
	"#   docs/site/export-and-build.sh              # 构建（跨面链/断链只告警；rc = mkdocs 的 rc）",
	"#   docs/site/export-and-build.sh --strict     # P1 验收形态：WARNING 即失败",
	"#   docs/site/export-and-build.sh --serve      # 构建后起本地服务 127.0.0.1:8000",
	"#   退码：0 全绿 · 1 构建有失败项 · 2 不给结论（缺件 / 空转 / 自检不过）",
}

// docSiteBuildVenvVerPy —— venv「实测版本」块（薄壳：查已装包元数据须那个解释器自身 · 照 sh 的内联块逐字）。
const docSiteBuildVenvVerPy = `import importlib.metadata as md
for p in ("mkdocs", "mkdocs-material", "mike", "jieba", "pymdown-extensions", "lunr"):
    try:
        print(f"  {p}=={md.version(p)}")
    except Exception as e:
        print(f"  {p}==? ({e})")
`

// dsbExitSpawnFail —— shell 惯例退码 127（mkdocs 子进程起不来 · 非 ExitError；正文由 sh 侧写进日志）。
// 属**底层壳件自有码空间**（✗ 不在命令面真源退码表 `core/cmd/zerg/exitcodes.go` ⇒ ✗ 写裸数字，门⑪ T3）；先例 = `deploy_launch.go`/`dplRCSSHAbsent` · `pack_release.go`/`prExitSpawnFail`（大写 E 不进门⑨ `exit…` regex）。
const dsbExitSpawnFail = 127

// cmdDocSiteBuild —— `zerg doc site build [--strict|--serve|--help]`（A14 的 Go 正门 · 真搬）。
//
// 退码口径（逐档照原脚本）：`0` 全绿 · `1` 构建有失败项（断链 / 跨面链 `--strict` 即红）· `2` 不给结论
// （缺件 / 空转 / 用法错 / 自检不过）。命令面自己只在「解析不到仓根」那一档退 `8`（不给结论 · Go 推导面
// 特有 · 见件头出厂声明 (a)）。
func cmdDocSiteBuild(inv *invocation, stdout, stderr io.Writer) int {
	tail := []string{}
	if len(inv.orig) > len(inv.path) {
		tail = inv.orig[len(inv.path):]
	}
	return docSiteBuildRun(inv, tail, stdout, stderr)
}

// docSiteBuildRun —— 真搬实现体：`args` = `zerg doc site build` 之后的实参（= 原脚本的 `"$@"`）。
func docSiteBuildRun(inv *invocation, args []string, stdout, stderr io.Writer) int {
	// ── 参数面（照 sh 的 for/case 逐字：按序判 · 命中 --help 立即退 0 · 未知立即退 2）──────────────
	strict, serve := false, false
	for _, a := range args {
		switch a {
		case "--strict":
			strict = true
		case "--serve":
			serve = true
		case "-h", "--help":
			fmt.Fprint(stdout, strings.Join(docSiteBuildHelpLines, "\n")+"\n")
			return exitOK
		default:
			fmt.Fprintf(stderr, "未知参数：%s（支持 --strict / --serve）\n", a)
			return exitUsage
		}
	}

	// ── 路径推导（sh 从 $0 上溯；Go 走 repoRoot —— 见件头出厂声明 (a)）──────────────────────────
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 取不了文档站路径（不给结论 · 退码 8）\n", progName)
		return exitBlocked
	}
	scriptDir := root + "/docs/site"
	docsSrc := root + "/docs"
	docsAlt := os.Getenv("ZERG_DOCS_ALT")
	if docsAlt == "" {
		docsAlt = root + "/../Zerg-内部文档"
	}
	confSrc := scriptDir + "/mkdocs.yml"
	req := root + "/tools/site-requirements.txt"
	venv := os.Getenv("ZERG_SITE_VENV")
	if venv == "" {
		venv = "/tmp/zerg-site-venv"
	}
	build := os.Getenv("ZERG_SITE_BUILD_DIR")
	if build == "" {
		build = "/tmp/zerg-site-build"
	}
	exportDir := build + "/export"
	logPath := build + "/build.log"

	// fail —— 照 sh 的 `fail() { echo "✗ $*" >&2; exit 2; }`。
	fail := func(msg string) int {
		inv.setErr("usage", "doc_site_build_failed", msg)
		fmt.Fprintf(stderr, "✗ %s\n", msg)
		return exitUsage
	}

	if !dsbIsFile(confSrc) {
		return fail("缺配置：" + confSrc)
	}
	if !dsbIsFile(req) {
		return fail("缺依赖清单：" + req)
	}
	if !dsbIsDir(docsSrc) {
		return fail("缺文档源：" + docsSrc)
	}

	// ── 找 python（venv 用 >=3.8）—— 薄壳：真跑候选解释器自报版本（外部工具）──────────────────────
	py := ""
	cands := []string{}
	if v := os.Getenv("ZERG_PYTHON"); v != "" {
		cands = append(cands, v)
	}
	cands = append(cands, "/opt/homebrew/bin/python3.13", "/opt/homebrew/bin/python3.12", "python3")
	for _, c := range cands {
		p, err := exec.LookPath(c)
		if err != nil || p == "" {
			continue
		}
		if exec.Command(p, "-c", "import sys; raise SystemExit(0 if sys.version_info >= (3, 8) else 1)").Run() == nil {
			py = p
			break
		}
	}
	if py == "" {
		return fail("找不到 python>=3.8（可用 ZERG_PYTHON=/path/to/python3 指定）")
	}
	pyVerOut, _ := exec.Command(py, "-V").CombinedOutput()
	pyVer := strings.TrimRight(string(pyVerOut), "\n")

	// ── 0/4 环境 ────────────────────────────────────────────────────────────────────────────────
	fmt.Fprintln(stdout, "=== 0/4 环境 ===")
	fmt.Fprintf(stdout, "仓根     : %s\n", root)
	fmt.Fprintf(stdout, "python   : %s (%s)\n", py, pyVer)
	fmt.Fprintf(stdout, "venv     : %s\n", venv)
	fmt.Fprintf(stdout, "构建目录 : %s\n", build)
	fmt.Fprintf(stdout, "文档源   : %s\n", docsSrc)
	if dsbIsDir(docsAlt) {
		if dsbIsDir(dsbPyJoin(docsAlt, "项目文档")) {
			fmt.Fprintf(stdout, "仓外取源 : %s（在位——分家件 项目文档/ 也在）\n", docsAlt)
		} else {
			fmt.Fprintf(stdout, "仓外取源 : %s（根在、但 项目文档/ 不在——分家未落盘时走仓内；两侧都缺 ⇒ rc=2）\n", docsAlt)
		}
	} else {
		fmt.Fprintf(stdout, "仓外取源 : %s（不存在——公开树/别的机器侧；仓内缺的页会 rc=2）\n", docsAlt)
	}

	// ── 1/4 venv + pin 依赖（薄壳：真装第三方包 · 联网）──────────────────────────────────────────
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "=== 1/4 venv + pin 依赖（tools/site-requirements.txt）===")
	venvPy := venv + "/bin/python"
	if !dsbIsExec(venvPy) {
		c := exec.Command(py, "-m", "venv", venv)
		c.Stdout, c.Stderr = stdout, stderr
		if err := c.Run(); err != nil {
			return fail("venv 创建失败（" + venv + "）")
		}
	}
	// pip 升级（sh：`>/dev/null 2>&1`，rc 不判）与装依赖（失败即 fail）。
	dsbRunSilent(venvPy, "-m", "pip", "install", "--quiet", "--disable-pip-version-check", "--upgrade", "pip")
	if err := dsbRunSilent(venvPy, "-m", "pip", "install", "--quiet", "--disable-pip-version-check", "-r", req); err != nil {
		return fail("装依赖失败（见 " + req + "；jieba 装不上 = §9.2 P1 的停手条件）")
	}
	fmt.Fprintln(stdout, "venv 实测版本：")
	vv := exec.Command(venvPy, "-")
	vv.Stdin = strings.NewReader(docSiteBuildVenvVerPy)
	vv.Stderr = stderr
	if out, _ := vv.Output(); len(out) > 0 {
		_, _ = stdout.Write(out)
	}

	// ── 2/4 产出导出树（只拷 nav 列出的正式面）──────────────────────────────────────────────────
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "=== 2/4 产出导出树（只拷 nav 列出的正式面）===")
	_ = os.RemoveAll(exportDir)
	_ = os.MkdirAll(exportDir, 0o755)
	_ = dsbCopyFile(confSrc, exportDir+"/mkdocs.yml")
	rcExport := dsbProduceExport(exportDir, docsSrc, docsAlt, stdout)
	if rcExport != 0 {
		return fail(fmt.Sprintf("导出树产出失败（rc=%d）", rcExport))
	}

	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "导出树一级内容：")
	ents, _ := os.ReadDir(exportDir)
	names := make([]string, 0, len(ents))
	for _, e := range ents {
		names = append(names, "./"+e.Name())
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Fprintf(stdout, "  %s\n", n)
	}

	// ── 3/4 mkdocs build（薄壳：站点工具链本体）─────────────────────────────────────────────────
	fmt.Fprintln(stdout)
	mode := "（非 strict）"
	if strict {
		mode = "--strict"
	}
	fmt.Fprintf(stdout, "=== 3/4 mkdocs build %s ===\n", mode)
	margs := []string{"build", "--config-file", exportDir + "/mkdocs.yml"}
	if strict {
		margs = append(margs, "--strict")
	}
	start := time.Now().Unix()
	mc := exec.Command(venv+"/bin/mkdocs", margs...)
	mc.Dir = exportDir
	logOut, runErr := mc.CombinedOutput()
	rcBuild := 0
	switch e := runErr.(type) {
	case nil:
	case *exec.ExitError:
		rcBuild = e.ExitCode()
	default:
		// sh 侧此处是 bash「command not found」⇒ 127；正文由 bash 写进日志（本件不复刻该正文）。
		rcBuild = dsbExitSpawnFail
		logOut = append(logOut, []byte(fmt.Sprintf("%s: %v\n", venv+"/bin/mkdocs", e))...)
	}
	end := time.Now().Unix()
	_ = os.WriteFile(logPath, logOut, 0o644)
	fmt.Fprintf(stdout, "mkdocs 原始输出（%s）：\n", logPath)
	fmt.Fprint(stdout, dsbSedPrefix(logOut, "  | "))
	fmt.Fprintf(stdout, "墙钟耗时（含 mkdocs 自报）：%d s\n", end-start)

	// ── 4/4 断言与计数 ──────────────────────────────────────────────────────────────────────────
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "=== 4/4 断言与计数 ===")
	site := exportDir + "/site"
	idx := site + "/search/search_index.json"
	rcAssert := dsbAssertSite(site, idx, stdout)

	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "=== 结果 ===")
	fmt.Fprintf(stdout, "  导出 rc  : %d\n", rcExport)
	fmt.Fprintf(stdout, "  构建 rc  : %d   （1 = 有失败项：断链/跨面链 --strict 即红）\n", rcBuild)
	fmt.Fprintf(stdout, "  断言 rc  : %d  （2 = 不给结论）\n", rcAssert)
	fmt.Fprintf(stdout, "  导出的树 : %s\n", exportDir)
	fmt.Fprintf(stdout, "  产物目录 : %s\n", site)
	fmt.Fprintf(stdout, "  构建日志 : %s\n", logPath)

	if serve {
		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, "=== 起本地服务（Ctrl-C 停）：http://127.0.0.1:8000 ===")
		sc := exec.Command(venv+"/bin/mkdocs", "serve", "--config-file", exportDir+"/mkdocs.yml")
		sc.Dir = exportDir
		sc.Stdin, sc.Stdout, sc.Stderr = os.Stdin, stdout, stderr
		if err := sc.Run(); err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				return ee.ExitCode()
			}
			return exitFail
		}
		return exitOK
	}
	if rcBuild != 0 {
		return rcBuild
	}
	return rcAssert
}

// docSiteBuildGateMain —— `cmdDocSiteBuild` 的 **inproc 同签名包装入口**（与 `gateCheckGoEntry` 表值
// 同签名：`func(stdout, stderr io.Writer, argv []string) int`）。
//
// ★ 只做 argv 归一（`argv` = `zerg doc site build` 之后的实参）后转发唯一实现 `cmdDocSiteBuild`
// ⇒ 两态三面与 `cmdDocSiteBuild` 逐字同（✗ 不重写任何一段、✗ 不改退码口径、✗ 不新增机器面）。
func docSiteBuildGateMain(stdout, stderr io.Writer, argv []string) int {
	inv := &invocation{
		path: []string{"doc", "site", "build"},
		orig: append([]string{"doc", "site", "build"}, argv...),
	}
	return cmdDocSiteBuild(inv, stdout, stderr)
}

// ── 真搬实现体：导出树产出（nav 即白名单）────────────────────────────────────────────────────────

// dsbProduceExport —— 原脚本第 114–227 行那枚内联 python 的真搬：
// 读导出树里的 `mkdocs.yml`（剥掉 mkdocs 专有标签）→ 解析 nav → 先仓内后仓外逐页取源拷入 → 生成占位首页
// → 非 UTF-8 检查 → 打印读数。任何缺失/坏件 ⇒ 退 2（不给结论）。返回「导出 rc」（0 / 1 / 2）。
func dsbProduceExport(exportDir, docsSrc, docsAlt string, stdout io.Writer) int {
	pages, perr := dsbNavPages(exportDir)
	if perr != nil {
		// py 侧此处抛未捕异常 ⇒ stderr=回溯 · rc=1（见件头「未覆盖面」）⇒ 本件降为「读/解析失败」退 1。
		return 1
	}
	if len(pages) == 0 {
		fmt.Fprintln(stdout, "✗ nav 解析出 0 个页面（白名单为空 ⇒ 空转，不许绿）")
		return 2
	}

	copied := []string{}
	generated := []string{}
	fromAlt := []string{}
	type dsbMiss struct{ rel, inRepo, altPath string }
	var missing []dsbMiss
	for _, rel := range pages {
		if rel == "index.md" {
			generated = append(generated, rel)
			continue
		}
		src := dsbPyJoin(docsSrc, rel)
		if !dsbIsFile(src) {
			alt := dsbPyJoin(docsAlt, rel)
			if dsbIsFile(alt) {
				src = alt
				fromAlt = append(fromAlt, rel)
			} else {
				missing = append(missing, dsbMiss{rel, dsbPyJoin(docsSrc, rel), alt})
				continue
			}
		}
		dst := dsbPyJoin(dsbPyJoin(exportDir, "docs"), rel)
		if err := dsbCopyFile(src, dst); err != nil {
			return 1
		}
		copied = append(copied, rel)
	}

	// 临时首页（生成物，不是正式面内容）：正式面真首页 index.md 是 W5 的产物。
	home := dsbPyJoin(dsbPyJoin(exportDir, "docs"), "index.md")
	lines := []string{
		"# 虫族文档（正式面）",
		"",
		"> **本页是构建期生成的临时首页**（W1 站点骨架的占位，勿手改）。",
		"> 正式面的真 `index.md` 是 W5 的产物：产出后把本段生成逻辑从",
		"> `docs/site/export-and-build.sh` 里去掉，改为拷入真首页即可。",
		"",
		fmt.Sprintf("正式面共 **%d** 篇（过程档案一条不进站）：", len(copied)),
		"",
	}
	for _, rel := range copied {
		base := filepath.Base(rel)
		base = strings.TrimSuffix(base, filepath.Ext(base))
		lines = append(lines, fmt.Sprintf("- [%s](%s)", base, rel))
	}
	lines = append(lines, "")
	if err := dsbWriteFile(home, strings.Join(lines, "\n")); err != nil {
		return 1
	}

	// 非 UTF-8 检查（实测：一处非 UTF-8 字节 = 全站构建失败、0 产物）。
	type dsbBad struct{ msg string }
	var bad []dsbBad
	mdsRoot := dsbPyJoin(exportDir, "docs")
	_ = filepath.Walk(mdsRoot, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() || !strings.HasSuffix(p, ".md") {
			return nil
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return nil
		}
		if !utf8.Valid(b) {
			bad = append(bad, dsbBad{fmt.Sprintf("%s: 'utf-8' codec can't decode（首个非法字节位置 %d）", p, dsbFirstBadByte(b))})
		}
		return nil
	})

	fmt.Fprintf(stdout, "  取源根            : %s\n", docsSrc)
	altState := "不存在"
	if dsbIsDir(docsAlt) {
		if dsbIsDir(dsbPyJoin(docsAlt, "项目文档")) {
			altState = "项目文档/ 在位"
		} else {
			altState = "根在 · 无 项目文档/"
		}
	}
	fmt.Fprintf(stdout, "  仓外取源          : %s（%s）\n", docsAlt, altState)
	head := fmt.Sprintf("  正文（nav 白名单）: %d 篇", len(copied))
	if len(fromAlt) > 0 {
		head += fmt.Sprintf("（其中仓外取源 %d 篇）", len(fromAlt))
	}
	fmt.Fprintln(stdout, head)
	fmt.Fprintf(stdout, "  生成首页（占位）  : %d 篇\n", len(generated))
	fmt.Fprintf(stdout, "  导出树 md 合计    : %d 篇\n", len(copied)+len(generated))
	if len(missing) > 0 {
		fmt.Fprintln(stdout, "✗ nav 列了、但两处取源根都没有（缺件 ⇒ rc=2 不给结论）：")
		for _, m := range missing {
			fmt.Fprintf(stdout, "    %s\n", m.rel)
			fmt.Fprintf(stdout, "      仓内：%s\n", m.inRepo)
			fmt.Fprintf(stdout, "      仓外：%s\n", m.altPath)
		}
	}
	if len(bad) > 0 {
		fmt.Fprintln(stdout, "✗ 非 UTF-8（会导致整站构建失败）：")
		for _, b := range bad {
			fmt.Fprintf(stdout, "    %s\n", b.msg)
		}
	}
	if len(missing) > 0 || len(bad) > 0 {
		return 2
	}
	fmt.Fprintln(stdout, "  ✓ 导出树 == nav（表外 md 一个都没进）")
	return 0
}

// ── 真搬实现体：断言与计数（原脚本第 254–289 行的内联 python）────────────────────────────────────

// dsbAssertSite —— 输入 md/产物 HTML 页数与体积 → 索引存在性 → `config.lang == ['zh']` →
// 索引条目含 U+200B。断言不过退 2；索引缺失退 2；解析失败退 1（见件头「未覆盖面」）。
func dsbAssertSite(site, idx string, stdout io.Writer) int {
	html, mds, siteBytes := 0, 0, int64(0)
	_ = filepath.Walk(site, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(p, ".html") {
			html++
		}
		siteBytes += info.Size()
		return nil
	})
	_ = filepath.Walk(dsbPyJoin(site, "..", "docs"), func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(p, ".md") {
			mds++
		}
		return nil
	})

	fmt.Fprintf(stdout, "  输入 md 页数   : %d\n", mds)
	fmt.Fprintf(stdout, "  产物 HTML 页数 : %d（含 404.html）\n", html)
	fmt.Fprintf(stdout, "  产物体积       : %s B（%.1f MiB）\n", dsbComma(siteBytes), float64(siteBytes)/1048576.0)

	if !dsbIsFile(idx) {
		fmt.Fprintf(stdout, "  ✗ 缺 %s（搜索索引未生成）\n", idx)
		return 2
	}
	blob, err := os.ReadFile(idx)
	if err != nil {
		fmt.Fprintf(stdout, "  ✗ 缺 %s（搜索索引未生成）\n", idx)
		return 2
	}
	var doc struct {
		Docs   []map[string]interface{} `json:"docs"`
		Config map[string]interface{}   `json:"config"`
	}
	if err := json.Unmarshal(blob, &doc); err != nil {
		fmt.Fprintf(stdout, "  ✗ 索引解析失败：%v\n", err)
		return 1
	}
	nZwsp := 0
	for _, x := range doc.Docs {
		t, _ := x["text"].(string)
		if strings.Contains(t, "\u200b") {
			nZwsp++
		}
	}
	sz := int64(0)
	if st, serr := os.Stat(idx); serr == nil {
		sz = st.Size()
	}
	fmt.Fprintf(stdout, "  索引体积       : %s B\n", dsbComma(sz))
	fmt.Fprintf(stdout, "  索引条目       : %d\n", len(doc.Docs))
	fmt.Fprintf(stdout, "  config.lang    : %s\n", dsbPyRepr(doc.Config["lang"]))

	rc := 0
	if !dsbLangIsZh(doc.Config) {
		fmt.Fprintln(stdout, "  ✗ 断言失败：config.lang != ['zh']（中文站必配 1/2，§5.3）")
		rc = 2
	} else {
		fmt.Fprintln(stdout, "  ✓ 断言通过：config.lang == ['zh']（必配 1/2）")
	}
	if nZwsp > 0 {
		fmt.Fprintf(stdout, "  ✓ 断言通过：%d/%d 条索引文本含 U+200B（jieba 生效，必配 2/2）\n", nZwsp, len(doc.Docs))
	} else {
		fmt.Fprintln(stdout, "  ✗ 断言失败：解析后无任何条目含 U+200B（jieba 没生效 = 必配 2/2）")
		rc = 2
	}
	return rc
}

// ── 小工具（前缀 dsb 防包内同名）────────────────────────────────────────────────────────────────

// dsbNavPages —— 读导出树的 `mkdocs.yml`（剥标签）→ 解析 → 收集 nav 里的 `*.md`（保文档序）。
func dsbNavPages(exportDir string) ([]string, error) {
	text, err := dsbReadStripTags(exportDir + "/mkdocs.yml")
	if err != nil {
		return nil, err
	}
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(text), &root); err != nil {
		return nil, err
	}
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return nil, fmt.Errorf("空配置")
	}
	nav := dsbMapValue(root.Content[0], "nav")
	if nav == nil {
		return nil, fmt.Errorf("nav 缺失")
	}
	var all []string
	dsbWalkPages(nav, &all)
	pages := make([]string, 0, len(all))
	for _, p := range all {
		if strings.HasSuffix(p, ".md") {
			pages = append(pages, p)
		}
	}
	return pages, nil
}

// dsbReadStripTags —— py `strip_tags()`：把 mkdocs 专有标签换成安全值，好让 yaml 能读 nav（只影响本次解析）。
func dsbReadStripTags(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var out strings.Builder
	for _, line := range strings.SplitAfter(string(b), "\n") {
		if line == "" {
			continue
		}
		s := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(s, "docs_dir:"):
			out.WriteString("docs_dir: docs\n")
		case strings.HasPrefix(s, "site_dir:"):
			out.WriteString("site_dir: site\n")
		case strings.Contains(line, "!!python/object/apply"):
			out.WriteString("      slugify: null\n")
		default:
			out.WriteString(line)
		}
	}
	return out.String(), nil
}

// dsbWalkPages —— py `walk()`：list 走项 · dict 走值（会话序）· str 收下（非字符串标量不收）。
func dsbWalkPages(n *yaml.Node, out *[]string) {
	if n == nil {
		return
	}
	switch n.Kind {
	case yaml.SequenceNode:
		for _, c := range n.Content {
			dsbWalkPages(c, out)
		}
	case yaml.MappingNode:
		for i := 1; i < len(n.Content); i += 2 {
			dsbWalkPages(n.Content[i], out)
		}
	case yaml.ScalarNode:
		if n.Tag == "!!str" {
			*out = append(*out, n.Value)
		}
	}
}

// dsbMapValue —— 在 YAML 映射节点里按键取值（找不到返回 nil）。
func dsbMapValue(n *yaml.Node, key string) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

// dsbPyJoin —— 照 CPython `posixpath.join` 的逐段规则直译（✗ 不用 filepath.Join：`..`/绝对段/空段语义不同）。
func dsbPyJoin(a string, p ...string) string {
	path := a
	for _, b := range p {
		switch {
		case strings.HasPrefix(b, "/"):
			path = b
		case path == "" || strings.HasSuffix(path, "/"):
			path = path + b
		default:
			path = path + "/" + b
		}
	}
	return path
}

// dsbPyRepr —— py `repr()` 的常用三形（None / 'str' / ['…'] 列表）；只服务 `config.lang` 那一格。
func dsbPyRepr(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return "None"
	case string:
		return "'" + t + "'"
	case bool:
		if t {
			return "True"
		}
		return "False"
	case float64:
		return strconv.FormatFloat(t, 'g', -1, 64)
	case []interface{}:
		parts := make([]string, 0, len(t))
		for _, e := range t {
			parts = append(parts, dsbPyRepr(e))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	default:
		return fmt.Sprintf("%v", t)
	}
}

// dsbLangIsZh —— `config.lang == ['zh']`（缺 config/缺 lang ⇒ 否）。
func dsbLangIsZh(cfg map[string]interface{}) bool {
	if cfg == nil {
		return false
	}
	l, ok := cfg["lang"].([]interface{})
	return ok && len(l) == 1 && l[0] == "zh"
}

// dsbComma —— py `{n:,}` 的千分位形态。
func dsbComma(n int64) string {
	s := fmt.Sprintf("%d", n)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteByte(s[i])
	}
	out := b.String()
	if neg {
		out = "-" + out
	}
	return out
}

// dsbSedPrefix —— 复刻 `sed 's/^/  | /'`：逐行加前缀（末行不完整也补一个换行 · 与 sed 同）。
func dsbSedPrefix(data []byte, prefix string) string {
	if len(data) == 0 {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	var b strings.Builder
	for _, ln := range lines {
		b.WriteString(prefix)
		b.WriteString(ln)
		b.WriteByte('\n')
	}
	return b.String()
}

// dsbFirstBadByte —— 第一个非法 UTF-8 字节的下标（供非 UTF-8 读数用）。
func dsbFirstBadByte(b []byte) int {
	for i := 0; i < len(b); {
		r, size := utf8.DecodeRune(b[i:])
		if r == utf8.RuneError && size == 1 {
			return i
		}
		i += size
	}
	return -1
}

// dsbIsFile / dsbIsDir / dsbIsExec —— 跟随符号链接（与 py 的 `os.path.isfile/isdir`、sh 的 `-f/-d/-x` 同）。
func dsbIsFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func dsbIsDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func dsbIsExec(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir() && st.Mode()&0o111 != 0
}

// dsbCopyFile —— `cp`/`shutil.copy2` 的内容搬运（建父目录 + 整件写）。
func dsbCopyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o644)
}

// dsbWriteFile —— 建父目录 + 整件写（生成物）。
func dsbWriteFile(p, content string) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(content), 0o644)
}

// dsbRunSilent —— 起子进程、吞掉两端输出（照 sh 的 `>/dev/null 2>&1`）。
func dsbRunSilent(name string, args ...string) error {
	c := exec.Command(name, args...)
	c.Stdout, c.Stderr = io.Discard, io.Discard
	return c.Run()
}
