// family_build.go —— 构建族（K 族）+ 「服务并入 `core`」（§3.5 · §7.1 `P11` · §17.2 ③ 建环）。
//
// 三条口径（照定稿，不自造）：
//
//	① **不新立 `svc` 族**（§7.1 `P11` 的推荐）：服务脚本面走 `zerg core daemon ls`
//	   （现跑覆盖 `scripts/svc/` 5 件）；构建面才叫 `build`。
//	② **制品矩阵的真源是脚本**：`scripts/build/build-all.sh` 是「怎么造出这些件」的唯一入口，
//	   本族**不复制**它的清单 —— `build ls` 只做**现读**（`bin/` 逐件 sha256 + `bin/build-info.json`
//	   的身份行），并写明「清单真源 = 那个脚本」。
//	③ `build all` / `build release` 会**覆盖正在跑的制品**（主控/子端）⇒ 那是**换件档**
//	   （不可逆动作的邻居）：本版**只登记形状、不执行**；判据「逐件同 sha」要**真跑一遍**才成立
//	   ⇒ 记为待拍（跑一次 = 覆盖 `bin/zerg-core` 等在跑的件）。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// buildArtifactRow —— `build ls` 的一行（现读值，不缓存、不估计）。
type buildArtifactRow struct {
	name string
	row  map[string]string
}

// cmdBuildLs —— `zerg build ls`（只读本机面）：`bin/` 逐件 sha256 + 身份件里的版本信息。
//
// 口径诚实说明：**这一条不等于「逐件同 sha」判据已经过** —— 它给的是**对拍用的现读值**；
// 判据要的是「`zerg build all` 产出的件与 `build-all.sh` 逐件同 sha」，那要**跑一次**，
// 而跑一次 = 覆盖在跑的制品（换件档）⇒ 待拍。
func cmdBuildLs(inv *invocation, stdout, stderr io.Writer) int {
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 列不出制品（不给结论 · 退码 8）\n", progName)
		return exitBlocked
	}
	binDir := filepath.Join(root, "bin")
	ents, err := os.ReadDir(binDir)
	if err != nil {
		inv.setErr("blocked", "bin_absent", "bin/ 读不到")
		fmt.Fprintf(stderr, "%s: `bin/` 读不到（%v）⇒ 不给结论（退码 8）\n", progName, err)
		return exitBlocked
	}
	identity := buildIdentity(binDir)
	rows := []map[string]string{}
	for _, e := range ents {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		p := filepath.Join(binDir, e.Name())
		fi, err := os.Stat(p)
		if err != nil || fi.Mode().IsDir() {
			continue
		}
		sum := ""
		if fi.Mode()&0o111 != 0 { // 可执行件才算「制品」；数据件（build-info.json 等）只列不签
			sum = fileSHA256(p)
		} else {
			sum = "（非可执行件 · 不计入制品）"
		}
		row := map[string]string{
			"name":    e.Name(),
			"sha256":  sum,
			"bytes":   fmt.Sprintf("%d", fi.Size()),
			"mtime":   fi.ModTime().Format("2006-01-02T15:04:05Z07:00"),
			"version": identity["version"],
			"code":    identity["code"],
			"built":   identity["built"],
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i]["name"] < rows[j]["name"] })
	if len(rows) == 0 {
		inv.setErr("blocked", "no_artifacts", "bin/ 里一件都没有")
		fmt.Fprintf(stderr, "%s: `bin/` 里没扫到件 ⇒ 不给结论（退码 8）\n", progName)
		return exitBlocked
	}
	fmt.Fprintf(stderr, "%s: 制品矩阵的**真源是脚本** `scripts/build/build-all.sh`（本命令只现读 `bin/`，不复制清单）\n", progName)
	fmt.Fprintf(stderr, "%s: 「逐件同 sha」判据要**跑一次**才成立 ⇒ 跑一次 = 覆盖在跑的制品（换件档）⇒ 待 Mr2109 拍\n", progName)
	return listCmd(inv, stdout, stderr, []string{"name", "sha256", "bytes", "version", "code", "built"}, rows)
}

// buildIdentity 读身份件（`bin/build-info.json`）——**只读**，取不到就留空（不猜版本）。
func buildIdentity(binDir string) map[string]string {
	out := map[string]string{"version": "", "code": "", "built": ""}
	b, err := os.ReadFile(filepath.Join(binDir, "build-info.json"))
	if err != nil {
		return out
	}
	var m map[string]any
	if json.Unmarshal(b, &m) != nil {
		return out
	}
	for _, k := range []string{"version", "code", "built", "built_at", "build_time", "commit", "git_sha", "sha"} {
		if v, ok := m[k].(string); ok {
			switch k {
			case "version":
				out["version"] = v
			case "code", "commit", "git_sha", "sha":
				if out["code"] == "" {
					out["code"] = v
				}
			case "built", "built_at", "build_time":
				if out["built"] == "" {
					out["built"] = v
				}
			}
		}
	}
	return out
}

// fileSHA256 现算一份件的 sha256（只读 · 不缓存）。
func fileSHA256(p string) string {
	f, err := os.Open(p)
	if err != nil {
		return "（读不到）"
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "（读失败）"
	}
	return hex.EncodeToString(h.Sum(nil))
}

// buildOnlyOpen —— 本版**唯一开放执行**的 `--only` 档：`cli`（只写 `bin/zerg` 一件）。
//
// 为什么只开这一档（窄口子，不是把闸拆了）：`build-all.sh --only-cli` 的件是**命令面自己的二进制**
// —— 不常驻、不绑端口、不在跑（`build-all.sh:28-31` 逐字：本档「只写 `bin/zerg` 一个文件，
// 其余制品与本脚本写的 `build-info.json` **一个字节都不动** ✓」）⇒ 它不是换件，是**自举**
// （改命令树要能重编命令面，否则「只走命令面」这条铁律在**改命令面自己**时会当场死锁）。
// `core`（覆盖**正在跑**的主控件）与 `compat` 仍是不可逆档 ⇒ 照旧**未开放**，要跑请 Mr2109 拍。
const buildOnlyOpen = "cli"

// buildOnlyLevels —— `--only` 的档闭集（**唯一真源**：用法串、错误文案、执行面都读它）。
// 档名逐字对齐 `scripts/build/build-all.sh` 的三枚旗标（`--only-cli`/`--only-core`/`--only-compat`）。
var buildOnlyLevels = []string{"cli", "core", "compat"}

// cmdBuildPassthrough —— `zerg build all` / `zerg build release`：登记形状 + **窄执行面**。
//
// 口径（与 D3b 前的唯一差别就是那一个窄口子）：
//
//	① `--dry-run`（默认三态里的第一态）：出计划件，零副作用 —— 不论哪一档一律 rc=0；
//	② **真跑只开一档**：`build all --only cli`（自举档 · 只写 `bin/zerg`）⇒
//	   D3 档确认（`--confirm=<主机名>` 与 `--yes` **同时到**），退码**直通脚本**（不翻译）；
//	③ 其余形态（`build all`（默认全套）· `--only core` · `--only compat` · `build release`）
//	   仍是换件/发布档 ⇒ **未开放**（rc=2 · 不给结论）。
func cmdBuildPassthrough(inv *invocation, stdout, stderr io.Writer) int {
	action := ""
	if len(inv.path) > 1 {
		action = inv.path[1]
	}
	only := strings.TrimSpace(inv.flagVal("--only"))
	if only != "" && !containsStr(buildOnlyLevels, only) {
		inv.setErr("usage", "bad_only_level", "`--only` 的档不在闭集里")
		fmt.Fprintf(stderr, "%s: `--only` 只认三档：%s（给了 %q）\n", progName, strings.Join(buildOnlyLevels, "|"), only)
		fmt.Fprintf(stderr, "档名逐字对齐构建脚本：--only-cli / --only-core / --only-compat\n")
		return exitUsage
	}
	script, effect := "scripts/build/build-all.sh", "重编**全部制品**（含正在跑的 zerg-core / zerg-agentd）并重签"
	if action == "release" {
		script, effect = "scripts/build/pack-release.sh", "打包发布件（布局三件：dist 目录 + 校验 + 清单）"
	}
	scriptArgs := []string{}
	openForExec := false
	if action == "all" && only == buildOnlyOpen {
		scriptArgs = []string{"--only-" + buildOnlyOpen}
		openForExec = true
		effect = "只写 `bin/zerg` 一件（命令面自举档）—— 其余制品与 `build-info.json` 一个字节不动"
	}
	if inv.dryRun {
		fmt.Fprintln(stdout, "计划件（--dry-run · 零副作用 —— 未执行、未改任何状态）")
		fmt.Fprintf(stdout, "  动作     : %s build %s%s\n", progName, action, onlyNote(only))
		fmt.Fprintf(stdout, "  危险档   : D3（换件档：会覆盖 `bin/` 里的在跑制品）\n")
		fmt.Fprintf(stdout, "  它会调   : bash %s %s\n", script, strings.Join(scriptArgs, " "))
		fmt.Fprintf(stdout, "  它会动   : %s\n", effect)
		fmt.Fprintf(stdout, "  执行要   : --confirm=<主机名> 与 --yes 同时到\n")
		if openForExec {
			fmt.Fprintf(stdout, "  本版状态 : **已开放**（自举档 —— 只写 bin/zerg，不是换件）\n")
		} else {
			fmt.Fprintf(stdout, "  更小的档 : `%s build all --only cli`（只写 bin/zerg 一件 · **本版唯一开放执行**的档）\n", progName)
			fmt.Fprintf(stdout, "  本版状态 : **未开放** —— 换件/发布属不可逆档；要跑请 Mr2109 拍（§6.3 S5/S7）\n")
		}
		fmt.Fprintf(stdout, "  来源     : §3.5 K 族 · §7.1 P11 · §17.2 ③ 建环 · 开工单 T-48 · 缺口面 P1（B1）\n")
		fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未执行、未改任何状态）")
		return exitOK
	}
	if !openForExec {
		inv.setErr("usage", "not_opened", "换件/发布档本版未开放")
		fmt.Fprintf(stderr, "%s: `build %s%s` 会覆盖 `bin/` 里**正在跑**的制品（换件档）⇒ 本版**未开放**（退码 2）\n",
			progName, action, onlyNote(only))
		fmt.Fprintf(stderr, "先看计划件：%s build %s%s --dry-run\n", progName, action, onlyNote(only))
		fmt.Fprintf(stderr, "本版唯一开放执行的档：`%s build all --only cli`（只写 bin/zerg 一件 · 自举档）\n", progName)
		return exitUsage
	}
	// ② 真跑（自举档）：D3 三态 —— `--confirm=<主机名>`（值必须与主机名逐字相同）与 `--yes` 同时到。
	host := planHost()
	if !inv.confirmGiven {
		msg := fmt.Sprintf("`build all --only %s` 会写 `bin/` ⇒ 按 D3 档：**缺确认 ⇒ 不执行**", buildOnlyOpen)
		inv.setErr("usage", "confirm_required", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		fmt.Fprintf(stderr, "要执行得给：--confirm=%s --yes\n", host)
		fmt.Fprintf(stderr, "先看计划件：%s build all --only %s --dry-run\n", progName, buildOnlyOpen)
		return exitUsage
	}
	if inv.confirm != host {
		msg := fmt.Sprintf("确认值不匹配目标（--confirm 给的是 %q，本机主机名是 %q）⇒ 不执行", inv.confirm, host)
		inv.setErr("usage", "confirm_mismatch", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		fmt.Fprintf(stderr, "`--confirm` 的值必须与目标**逐字相同**（§4.1 K7）—— 给错值一律拒绝，不许「当没给」\n")
		return exitUsage
	}
	if !inv.yes {
		msg := fmt.Sprintf("`build all --only %s` 是 D3 档 —— **缺 --yes ⇒ 不执行**", buildOnlyOpen)
		inv.setErr("usage", "yes_required", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		return exitUsage
	}
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 跑不动构建（不给结论 · 退码 8）\n", progName)
		return exitBlocked
	}
	if _, err := os.Stat(filepath.Join(root, script)); err != nil {
		inv.setErr("blocked", "script_absent", "构建脚本不在")
		fmt.Fprintf(stderr, "%s: 构建脚本不在（%s）⇒ 不给结论（退码 8）\n", progName, script)
		return exitBlocked
	}
	fmt.Fprintf(stderr, "%s: 自举档执行 —— bash %s %s（只写 bin/zerg 一件）\n", progName, script, strings.Join(scriptArgs, " "))
	cmd := exec.Command("bash", append([]string{filepath.Join(root, script)}, scriptArgs...)...)
	runningChild = cmd
	defer func() { runningChild = nil }()
	cmd.Dir = root
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			// ★ 只转发、不翻译：脚本的码原样返回（与 gate 族同一条口径 · §4.3 U1）。
			fmt.Fprintf(stderr, "%s: 构建脚本退 %d（原样转发，不翻译）\n", progName, ee.ExitCode())
			return ee.ExitCode()
		}
		inv.setErr("failed", "build_script_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 跑不动构建脚本：%v\n", progName, err)
		return exitFail
	}
	return exitOK
}

// onlyNote —— 把 `--only <档>` 写成可读后缀（空档 ⇒ 空串）。
func onlyNote(only string) string {
	if only == "" {
		return ""
	}
	return " --only " + only
}
