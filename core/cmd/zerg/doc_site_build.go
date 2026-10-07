// doc_site_build.go —— 文档站「导出 + 构建」的 **CLI 正门**（`zerg doc site build`）· 壳转账薄壳（只转发、不翻译）。
//
// 依据：`~/.zerg/state/A档18件-收CLI设计稿-20261007.md` 批 2 · A14
//
//	（`docs/site/export-and-build.sh` · 308 行 · 现属面＝文档站面 · 归族 = 现役 `doc` 族扩 action）。
//
// 落法 **(b) 壳转账**（设计稿逐字）：底层唯一实现 = `docs/site/export-and-build.sh`（真跑
// venv / pip / `mkdocs build` 站点工具链 · 联网 + 子进程 · OS 原生面 ⇒ 无法干净真 Go 化）；
// 命令面**只做入口 ＋ 门禁 ＋ 转发**，✗ 不重写脚本里的任何一段（否则就长出第二个真源 ——
// 与 A4 `build_wall.go` 同一条纪律）。
//
// ★ M2 纪律：本件**不接线**（✗ 不在 `main.go` 命令表登记 `doc site build`、✗ 不改
// `family_doc.go`、✗ 不改任何 `add_step`）。登记那一条（`path: []string{"doc","site","build"};
// passthrough: true; run: cmdDocSiteBuild`）要改 `main.go` ＝ **另一路独占**（派单 ④）
// ⇒ 本件先落实现、接线延后；在此之前 `cmdDocSiteBuild` 无调用点。
//
// 三条口径（照仓里既有的透传族写 —— `build_wall.go` / `gate` 族，不自己创式样）：
//
//	① **只转发、不翻译**：`zerg doc site build` 之后的**一切**（`--strict` / `--serve` /
//	    `--help` / 未知参数）原样交给脚本，连退码也不改写（0 / 2 逐档原样转出）。底层逻辑
//	    只有一处 ⇒ 本件不许重写任何一段 ✗。
//	② **命令面自己只在「连脚本都递不到」那一档退码**：解析不到仓根 / 脚本不在 ⇒ `8`
//	    （不给结论 · 与 `doc` 族同一档）；**不拿脚本的 2 冒充**（两件事不许并成一个形状）。
//	③ **不新增机器面**：底层脚本没有 `--json` ⇒ 本件也不自己拼一份包封。
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// docSiteBuildScriptRel —— 文档站导出+构建的**唯一实现**（真源）。本件一个字都不重写它：
// 旗标面（`--strict` / `--serve` / `--help`）、读数（venv / 页数 / 体积）、退码（0 / 1 / 2）全在那边。
const docSiteBuildScriptRel = "docs/site/export-and-build.sh"

// cmdDocSiteBuild —— `zerg doc site build [--strict|--serve|--help]`（A14 的 Go 正门 · 只转发不翻译）。
//
// 退码口径（**逐档照底层**）：脚本的 `0` / `1` / `2` 原样转出 ——
//
//	0 全绿 · 1 构建有失败项（断链 / 跨面链 --strict 即红）· 2 不给结论（缺件 / 空转 / 用法错）。
//
// 命令面自己只在「递不到脚本」那一档退码：解析不到仓根 / 脚本不在 ⇒ `8`（不给结论 · 与 `doc` 族同档）。
func cmdDocSiteBuild(inv *invocation, stdout, stderr io.Writer) int {
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 递不到文档站导出+构建脚本（不给结论 · 退码 8）\n", progName)
		return exitBlocked
	}
	script := filepath.Join(root, docSiteBuildScriptRel)
	if _, err := os.Stat(script); err != nil {
		inv.setErr("blocked", "doc_site_build_script_absent", "文档站导出+构建脚本不在")
		fmt.Fprintf(stderr, "%s: 文档站导出+构建脚本不在（%s）⇒ 不给结论（退码 8）\n", progName, docSiteBuildScriptRel)
		return exitBlocked
	}
	// 透传面：`zerg doc site build` 之后的**一切**原样交给脚本。
	// `doc` 族占三个路径段（`doc` ＋ `site` ＋ `build`）⇒ 取 `inv.orig[len(inv.path):]`，
	// 与 `build_wall.go` 同一取法。
	tail := []string{}
	if len(inv.orig) > len(inv.path) {
		tail = inv.orig[len(inv.path):]
	}
	fmt.Fprintf(stderr, "%s: 底层 = %s（**唯一真源** · 命令面只转发、不翻译）\n", progName, docSiteBuildScriptRel)
	cmd := armChildGroup(exec.Command("bash", append([]string{script}, tail...)...))
	runningChild = cmd
	defer func() { runningChild = nil }()
	cmd.Dir = root
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			// ★ 只转发、不翻译：脚本的码原样返回（与 `gate`/`doc` 族同一条口径）。
			fmt.Fprintf(stderr, "%s: 文档站导出+构建脚本退 %d（原样转发，不翻译）\n", progName, ee.ExitCode())
			return ee.ExitCode()
		}
		inv.setErr("failed", "doc_site_build_script_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 起不动文档站导出+构建脚本：%v\n", progName, err)
		return exitFail
	}
	return exitOK
}

// docSiteBuildGateMain —— `cmdDocSiteBuild` 的 **inproc 同签名包装入口**（与 `gateCheckGoEntry` 表值
// 同签名：`func(stdout, stderr io.Writer, argv []string) int`）。
//
// ★ **仅转发**：把 argv 原样当作 `zerg doc site build` 之后的实参，转交 `cmdDocSiteBuild`
// （唯一实现仍是 `cmdDocSiteBuild` ＋ 底层脚本 `docs/site/export-and-build.sh` —— 本件 ✗ 不重写
// 任何一段、✗ 不改退码口径、✗ 不新增机器面；两态三面与 `cmdDocSiteBuild` 逐字同）。
// ★ M2 纪律：本入口**暂不接线**（✗ 未在 `core/cmd/zerg/gate_inproc_table.go` 登记键 —— 表键另路一并接）。
func docSiteBuildGateMain(stdout, stderr io.Writer, argv []string) int {
	inv := &invocation{
		path: []string{"doc", "site", "build"},
		orig: append([]string{"doc", "site", "build"}, argv...),
	}
	return cmdDocSiteBuild(inv, stdout, stderr)
}
