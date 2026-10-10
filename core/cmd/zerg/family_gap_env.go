// family_gap_env.go —— gap 族环境旋钮三档 + 状态目录可视输出（缺口 `GAP-20260927-75` · 2026-10-10）。
//
// 病（缺口逐字）：别的会话把 `ZERG_STATE_DIR` 留在环境里指向沙箱目录（`…/tmp-…/…`）时，
// `zerg gap ls` **静默**读错状态目录（退 8）—— 一个字都不提「状态目录被环境变量改了」；
// 同类旋钮在门侧也会静默走差路（48 处无默认值读取）。
//
// 治（照门侧**同规三档** · `scripts/gates/precommit-gates.sh:238-280` 的「夹具口 / 仓根 / 其余」）：
//
//	① 夹具专用口  —— 只被自检 / 门禁夹具设置、设了就换真源且**无默认回落**的旋钮
//	   ⇒ **无条件清 + 清理可见**（✗ 不许「设了就换真源、不设就静默回落」那种静默换源）。
//	   清单与门侧 `GATE_SEALED_ENV` **同集**（同一份「夹具专用」口径）。
//	② 仓根旋钮    —— `ZERG_REPO` / `ZERG_REPO_ROOT` ⇒ **显式申报**真仓根 + 来源
//	   （✗ 不静默采信继承值：指到非仓根 ⇒ 逐字报「取不到」，与 `resolveRepoRoot` 的 fail-closed 同源）。
//	③ 其余正式旋钮 —— `ZERG_STATE_DIR` ⇒ **来源申报**（固定格式 · 机读令牌闭集），
//	   并在 gap 族命令上给出**当前状态目录**的可视输出。
//
// ★ 落点：`main.go` 的 `run()` 在 `cmd.run` **之前**、对 `cmd.path[0]=="gap"` 调**一次**
// （**唯一咽喉点** ⇒ 覆盖全 gap 族，不逐个命令铺；`--help` 那一路已在其前返回 ⇒ 不掺进帮助面）。
//
// ★★ 两条落点纪律（都是被既有契约逼出来的，不是风格）：
//
//   - **只写 stderr、不碰 stdout 一个字节**：同族先例 = approve 族 `GAP-20260927-199` 的
//     `printApproveStateDir` —— 矩阵 `want_stdout_bytes` / `ls` 表网格 / `--json` 包封逐字不动。
//   - **申报行攒着、命令跑完后追加到 stderr 尾**（本函数**返回**那几行，由 `run()` 收尾写出）：
//     gap 族的 stderr **首行**是**判词契约**（`cli_gap_reason_test` / `cli_gap_test` 逐字钉住
//     「error.kind=…」/「账内越界：…」）⇒ 申报行若顶到前面，既有判据件当场红。
//
// ★ 机读：申报行**固定格式**、来源写**闭集令牌**（`env:ZERG_STATE_DIR` / `default:~/.zerg/state`、
// 仓根来源写 `resolveRepoRoot` 的 `via` 闭集）⇒ 机可 grep，不靠散文。
package main

import (
	"fmt"
	"os"
	"strings"
)

// gapFixtureEnvKnobs —— ① 夹具专用口：与门侧 `precommit-gates.sh` 的 `GATE_SEALED_ENV` **同集**。
// 这些旋钮 `getenv` 有值就用、不报错（= 静默换真源）⇒ gap 族入口**无条件清**并打印清面。
var gapFixtureEnvKnobs = []string{
	"ZERG_SVCDECL_DECL", "ZERG_SVCDECL_PS", "ZERG_SVCDECL_LAUNCHCTL", "ZERG_SVCDECL_HOME",
	"ZERG_ALLSH_UNITTEST", "ZERG_REAL_GATES_UNITTEST", "CHECK_DOC_NAME_META_OFF",
}

// 状态目录来源的**两值闭集**（机读令牌 · 固定串，不是散文）。
const (
	gapStateDirSrcEnv     = "env:ZERG_STATE_DIR"
	gapStateDirSrcDefault = "default:~/.zerg/state"
)

// gapStateDirNote —— 状态目录 + 来源（**唯一取值口** · 与 `stateDirOf()` 同口径）。
func gapStateDirNote() (dir, source string) {
	if strings.TrimSpace(os.Getenv("ZERG_STATE_DIR")) != "" {
		return stateDirOf(), gapStateDirSrcEnv
	}
	return stateDirOf(), gapStateDirSrcDefault
}

// prepGapFamilyEnv —— gap 族入口**三档处理**（唯一落点 · 由 `run()` 在 `cmd.run` 之前调）。
//
// 副作用：① 夹具专用口**当场**无条件清（命令启动前就生效 ⇒ 命令读不到那批旋钮）。
//
// 返回值：②③ 的**申报行**（攒着，由 `run()` 在命令跑完后**追加到 stderr 尾** ——
// 见件头「两条落点纪律」，✗ 不顶掉 gap 族的首行判词）。本函数**不改任何判定、不动退码**。
func prepGapFamilyEnv() []byte {
	var b strings.Builder
	// ① 夹具专用口：无条件清 + 清理可见（✗ 不静默换源 —— 设了就点名、清了就说）
	var cleared []string
	for _, k := range gapFixtureEnvKnobs {
		if _, ok := os.LookupEnv(k); ok {
			_ = os.Unsetenv(k)
			cleared = append(cleared, k)
		}
	}
	if len(cleared) > 0 {
		fmt.Fprintf(&b, "%s: ① 夹具专用口无条件清（清理可见）：%s\n",
			progName, strings.Join(cleared, "、"))
	}
	// ② 仓根旋钮：显式申报真仓根 + 来源（✗ 不静默采信继承值）
	root, via, why := resolveRepoRoot()
	if root != "" {
		fmt.Fprintf(&b, "%s: ② 仓根 = %s（来源：%s）\n", progName, root, via)
	} else {
		fmt.Fprintf(&b, "%s: ② 仓根取不到 —— %s\n", progName, why)
	}
	// ③ 其余正式旋钮：状态目录来源申报 + 可视输出（固定格式 · 机读）
	dir, source := gapStateDirNote()
	fmt.Fprintf(&b, "%s: ③ 状态目录 = %s（来源：%s）\n", progName, dir, source)
	return []byte(b.String())
}
