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
// ③ **缺件即硬错**（脚本最小入口不在 = 自举件缺席，不给结论，退码 8）。
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
	script := filepath.Join(root, i18nScriptRel)
	if _, err := os.Stat(script); err != nil {
		inv.setErr("blocked", "i18n_script_absent", "i18n 门禁脚本最小入口不在")
		fmt.Fprintf(stderr, "%s: i18n 门禁脚本最小入口不在（%s）—— 它是自举件，就地缺席即硬错（退码 8）\n", progName, i18nScriptRel)
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
		tail = inv.orig[len(inv.path):]
	}
	return i18nAuditGateMain(stdout, stderr, tail)
}
