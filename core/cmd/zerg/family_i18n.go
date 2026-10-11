// family_i18n.go —— `zerg ui i18n`（多语言决策⑥ 2026-09-11 · i18n 门禁命令化）。
//
// 为什么它要进命令面：`ui/scripts/check-i18n.py` 是 CI 与发布导出都会跑的门禁（四道门：
// G1 键对称 / G2 引用完整 / G3 占位符一致 / G4 中文门禁），失败即中止发布。今天它只靠
// `python3 …/check-i18n.py` 手搓，退码、stdout/stderr 全靠人盯 —— 命令化之后「脚本退多少、
// 命令面就退多少」这条薄壳口径成了机器可判的硬约束（同 `gate …` 透传不翻译 §九 M11 `G1-a`）。
//
// 口径（照 §十一 命令面形态，不自造）：
//
//	形态 `zerg ui i18n [--json <字段>]`
//	逐字透传脚本的 stdout/stderr（人面表格 / 机器面六键包封都走 listCmd）
//	退码 `0` 通过 / `1` 门禁失败（白名单超差，即脚本的退码 —— **不许吞**）/ `2` 用法错 / `8` 脚本缺
//
// 三条口径：① **直通不翻译**（脚本退 1 本命令必非 0，薄壳只转发不改写）；
// ② **stdout/stderr 原样**（脚本自己的结论逐字转出，命令面不二次加工）；
// ③ **两半皆缺才硬错**（三段式回退腿：py 在盘优先 ⇒ 缺席回退 Go 半 `ci18Run`/`checkI18nGateMain` ⇒ 两半皆缺才判不了，不给结论、退码 8；✗ **不把「py 件不在盘」当成失败**）。
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// i18nFields —— `--json` 可取字段（闭集；不给字段时 stderr 列的就是它）。
var i18nFields = []string{"rc", "verdict"}

// i18nScriptRel —— 门禁脚本相对仓根的路径（红线：不许写死私有绝对路径，走 repoRoot 推导）。
const i18nScriptRel = "ui/scripts/check-i18n.py"

// i18nGoHalfRel —— i18n 门禁脚本（`ui/scripts/check-i18n.py`）的 **Go 半**（本仓真搬体 `check_i18n.go`）。
// 三段式回退腿：py 在盘优先 · 缺席回退 Go 半（`ci18Run` / `checkI18nGateMain`）· 两半皆缺才判不了（fail-closed）。
// 先例：`check_private_path_forms.go` 的 `pvGoHalfRel` · `check_newfile_registry.go` 的 `gnrLoadWording`。
const i18nGoHalfRel = "core/cmd/zerg/check_i18n.go"

// i18nIsFile —— 存在且是普通文件（退役判定的两缀之一 · 与同族先例同形）。
func i18nIsFile(root, rel string) bool {
	st, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
	return err == nil && st.Mode().IsRegular()
}

func cmdI18n(inv *invocation, stdout, stderr io.Writer) int {
	// ★ 用法错**先判**：`--json` 不给字段 ⇒ 当场 1 + stdout 0 字节（§4.1 K2）—— 不许先真跑门禁。
	if inv.jsonGiven && len(inv.fields) == 0 {
		if rc := requireFields(inv, stderr); rc != 0 {
			return rc
		}
	}
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 —— 跑门禁需要仓根（找 core/internal/version/version.go 失败）\n", progName)
		fmt.Fprintf(stderr, "在仓内跑，或设 ZERG_REPO=<仓根>\n")
		return exitBlocked
	}
	// ★ 三段式回退腿（先例 `check_private_path_forms.go` 的 `pvGoHalfRel` / `check_newfile_registry.go`
	//   的 `gnrLoadWording`）：py 在盘 ⇒ 照旧（后端已是 Go 半 `ci18Run` · 逐字不变）；
	//   py 缺席而其 **Go 半**在盘 ⇒ 退役态（门禁改由 Go 半承载 · **照常跑** · ✗ 不判「缺件」）；
	//   两半皆缺（真消失）⇒ 判不了（fail-closed · 退码 8，原语义保留）。
	if !i18nIsFile(root, i18nScriptRel) && !i18nIsFile(root, i18nGoHalfRel) {
		inv.setErr("blocked", "i18n_script_absent", "i18n 门禁脚本最小入口不在")
		fmt.Fprintf(stderr, "%s: i18n 门禁脚本最小入口不在（%s · 其 Go 半 %s 亦不在盘）—— 判不了（退码 8）\n", progName, i18nScriptRel, i18nGoHalfRel)
		return exitBlocked
	}

	// 真跑门禁：后端 = Go 真搬体 `ci18Run`（= `checkI18nGateMain` 的实现体 · 与 py `ui/scripts/check-i18n.py` 的
	// M2 真搬逐字对齐）。★ **只换执行核**：根仍走 `repoRoot()`（与脚本路径同源 · ✗ 不改宽语义），stdout/stderr
	// 透传与退码口径**一字不动**（脚本退多少 ⇒ 命令面退多少 · §九 M11 `G1-a`）。
	// 无 --json：stdout/stderr 原样透传；带 --json：输出收进缓冲（人读表格不许混进机器面），随后出六键包封。
	var buf bytes.Buffer
	runOut, runErrW := io.Writer(stdout), io.Writer(stderr)
	if inv.jsonGiven {
		runOut, runErrW = &buf, &buf
	}
	rc := ci18Run(root, runOut, runErrW, nil)
	if rc != 0 {
		fmt.Fprintf(stderr, "%s: 门禁失败（脚本退 %d）—— 白名单超差，修红再看\n", progName, rc)
	} else {
		fmt.Fprintf(stderr, "%s: 门禁通过（脚本退 0）\n", progName)
	}

	// 机器面：显式要了字段才出（`--json` 不给字段已在上方用法错里挡掉）。
	if inv.jsonGiven {
		verdict := "pass"
		if rc != 0 {
			verdict = "fail"
		}
		rows := []map[string]string{{"rc": fmt.Sprintf("%d", rc), "verdict": verdict}}
		if nrc := listCmd(inv, stdout, stderr, i18nFields, rows); nrc != 0 {
			return nrc
		}
	}
	return rc
}

// ── S7 · 5 键归口（ui 族扩容）：`zerg ui i18n-audit` 的**转发壳** ─────────────────────────────────
//
// 归口件：`~/.zerg/state/②类-5键-CLI工具子命令归口-20261007.md`。底层唯一实现 = `i18n_audit.go`
// （`ui/scripts/i18n_audit.py` 的 Go 真搬体）；本处只做**入口 ＋ 转发**：把动作名之后的一切（模式位置参数
// 与旗标逐字）原样交给 `i18nAuditGateMain`，连退码也不改写。✗ 不重写真搬体里任何一段。
func cmdI18nAudit(inv *invocation, stdout, stderr io.Writer) int {
	tail := []string{}
	if len(inv.orig) > len(inv.path) {
		tail = stripGlobalLeak(inv.orig[len(inv.path):])
	}
	return i18nAuditGateMain(stdout, stderr, tail)
}
