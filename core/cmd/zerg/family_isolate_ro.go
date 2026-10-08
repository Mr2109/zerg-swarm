// family_isolate_ro.go —— 只读隔离的**顶层命令**（`zerg isolate-ro new|verify` · 根因 `GAP-20260926-20` P0 · 2026-09-26）。
//
// 病灶（事故形态 · 逐字留档）：一枚卵为了「隔离验证」自建临时目录 + **符号链接**农场，
// `ln -s <真仓>/core/cmd/zerg/testdata <农场>/…/testdata` 之后 `rm -f <农场>/…/cli-matrix.json`
// —— 删掉的是**真仓的件**（链接把删除动作透传回去了）。真因不是手滑，是**没有「只读隔离」的正门**。
//
// ★ 2026-10-08 **真搬接线**（本单）：本命令**不再 exec** `scripts/dev/isolate-ro.sh`
// —— 该脚本的实际行为已收进 **Go 真身** `isolate_ro.go`（入口 `isolateRoGateMain`）。
// 命令面（本件）只做**入口 ＋ 转发**：`zerg isolate-ro` 之后的一切（动作、位置参数、旗标
// 与它们的值）原样交给 Go 真身，✗ 不重写它任何一段（否则就长出第二个真源）。
//
// ★ **同源入口**：`zerg isolate-ro`（本件）与 `zerg gate check isolate-ro`（进程内判据）走的是
// **同一个 Go 真身** `isolateRoGateMain` ⇒ `scripts/dev/isolate-ro.sh` 退役（移出仓）后，本命令
// 的 rc/stdout/stderr 与在盘时**逐字同**（✗ 不再退 `8`）。
//
// 退码口径（**逐档照 Go 真身** · 与脚本直跑逐字同）：0 正常 · 1 危险形态（拒绝的符号链接 /
// 对拍有差异）· 2 **不给结论**（用法错 / 缺件 / 空转 / 自检不过）。
// 命令面自己只在「解析不到仓根」那一档退码 `8`（不给结论 · 与 `ui` / `build` 族同档）。
package main

import (
	"fmt"
	"io"
)

// cmdIsolateRO —— `zerg isolate-ro <动作> [参数] [旗标]`（动作：new | verify；`--self-test` 自检）。
//
// ★ 只做入口：转发给 **Go 真身** `isolateRoGateMain`（`isolate_ro.go` · 亦即 `zerg gate check
// isolate-ro` 的进程内入口）—— 旗标面（`--dst` / `--include` / `--exclude` / `--all` /
// `--reject-symlinks` / `--no-manifest` / `--quiet` / `--max-examples` / `--self-test`）、
// 读数、退码全在那边。
//
// 取参口径照旧：顶层命令只占一个路径段 ⇒ 取 `inv.orig[1:]`（多切一段就把动作 `new` / `verify`
// 吃掉）；位置参数的次序、`--include a --include b` 的重复、`--dst <目录>` 的值都逐字过去。
//
// ★ 相对路径实参按**仓根**解析 —— 脚本当年以 `cmd.Dir = root` 起，故 Go 真身也在**仓根域内**
// 跑（复用判据侧同一 cwd/env 口径 `gateCheckRunInproc` · 与 `gate check isolate-ro` 逐字同源）。
func cmdIsolateRO(inv *invocation, stdout, stderr io.Writer) int {
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 递不到只读隔离（不给结论 · 退码 8）\n", progName)
		return exitBlocked
	}
	tail := []string{}
	if len(inv.orig) > 1 {
		tail = inv.orig[1:]
	}
	return gateCheckRunInproc(isolateRoGateMain, root, stdout, stderr, tail)
}
