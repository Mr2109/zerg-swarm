// family_build.go —— 构建族（K 族）+ 「服务并入 `core`」（§3.5 · §7.1 `P11` · §17.2 ③ 建环）。
//
// 三条口径（照定稿，不自造）：
//
//	① **不新立 `svc` 族**（§7.1 `P11` 的推荐）：服务脚本面走 `zerg core daemon ls`
//	   （现跑覆盖 `scripts/svc/` 5 件）；构建面才叫 `build`。
//	② **制品矩阵的真源是构建器**：Go 真身 `scripts/build/cmd/zerg-build-all/main.go`（真搬自壳脚本 `scripts/build/build-all.sh`）是「怎么造出这些件」的唯一入口，
//	  本族**不复制**它的清单 —— `build ls` 只做**现读**（`bin/` 逐件 sha256 + **逐件**身份
//	  （读制品自身嵌入段 `commit`/`build_time`）＋ 整批版本（`bin/build-info.json`）），并写明「清单真源 = 那个构建器」。
//	③ `build all` / `build release` 会**覆盖正在跑的制品**（主控/子端）⇒ 那是**换件档**
//	   （不可逆动作的邻居）：本版**只登记形状、不执行**；判据「逐件同 sha」要**真跑一遍**才成立
//	   ⇒ 记为待拍（跑一次 = 覆盖 `bin/zerg-core` 等在跑的件）。
package main

import (
	"crypto/sha256"
	"debug/buildinfo"
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

// cmdBuildLs —— `zerg build ls`（只读本机面）：`bin/` 逐件 sha256 + **逐件**身份（读制品自身嵌入段
// `commit`/`build_time`）＋ 整批版本（`bin/build-info.json`）。
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
	// version = **整批聚合版本**（嵌入段不含 version —— 见 `buildIdentity` 注）；逐件的 code/built
	// 在下面对每一件**各自现读**（✗ 不套同一份）。
	version := buildIdentity(binDir)["version"]
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
		code, built := artifactEmbeddedIdentity(p)
		row := map[string]string{
			"name":    e.Name(),
			"sha256":  sum,
			"bytes":   fmt.Sprintf("%d", fi.Size()),
			"mtime":   fi.ModTime().Format("2006-01-02T15:04:05Z07:00"),
			"version": version,
			"code":    code,
			"built":   built,
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i]["name"] < rows[j]["name"] })
	if len(rows) == 0 {
		inv.setErr("blocked", "no_artifacts", "bin/ 里一件都没有")
		fmt.Fprintf(stderr, "%s: `bin/` 里没扫到件 ⇒ 不给结论（退码 8）\n", progName)
		return exitBlocked
	}
	fmt.Fprintf(stderr, "%s: 制品矩阵的**真源是构建器** Go 真身 `scripts/build/cmd/zerg-build-all/main.go`（真搬自壳脚本 `scripts/build/build-all.sh`；本命令只现读 `bin/`，不复制清单）\n", progName)
	fmt.Fprintf(stderr, "%s: 身份**读制品自身**（设计 §5 `O-12` 取（乙） · 2026-09-24 待拍清单终版 条 4/5）：`code`/`built` 逐件读制品自身嵌入段 `commit=<sha>` / `build_time=<ts>`（`debug/buildinfo` 只读文件头 ⇒ **不执行该件**也读得到）⇒ 「逐件同 sha」判据不再需要（甲）档那条「跑一次 = 覆盖在跑的制品（换件档）」；读它**只读**（负控：制品 `sha256` 跑前跑后不变）。读不到嵌入段（非 Go 制品 / 无该二标签的老件）⇒ 如实示「（无嵌入标签）」（✗ 不回落成 `bin/build-info.json` 的同名值）；`version` 是**整批聚合**版本，仍取 `bin/build-info.json`（嵌入段不含 version）\n", progName)
	return listCmd(inv, stdout, stderr, []string{"name", "sha256", "bytes", "version", "code", "built"}, rows)
}

// buildIdentity 读身份件（`bin/build-info.json`）——**只读**，取不到就留空（不猜版本）。
//
// ★ 2026-10-05（缺口 `GAP-20261005-24`）：本函数**只**供 `version` 用（整批聚合版本）。
//
//	`code`/`built` 已改由 `artifactEmbeddedIdentity` **逐件**读制品自身嵌入段 ——
//	`build-info.json` 是**整批**的聚合描述（只有一个值），套到每件上会让老备份/异批次件「读错代」
//	（拿它做「换件后验：在跑的件 == 盘上件」会读到错的代）。
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

// buildEmbeddedNone —— 读不到嵌入段时 `code`/`built` 的**如实**落点（✗ 不回落成同名值）。
const buildEmbeddedNone = "（无嵌入标签）"

// artifactEmbeddedIdentity 读**该件自身**的嵌入段身份（`-tags commit=<sha>,build_time=<ts>`）。
//
// 真源 = 制品自身（设计 §5 `O-12` 取（乙） · 缺口 `GAP-20261005-24`）：
//   - 用 `debug/buildinfo` 读**文件头**（不执行该件 · 只读 · 制品 `sha256` 不变）；
//   - 标签形态与 `go version -m <件>` 同源（`build\t-tags=commit=…,build_time=…`）；
//   - 读不到（非 Go 制品 / 无该二标签的老件）⇒ 两格都如实示 `buildEmbeddedNone`
//     （✗ **不回落成** `bin/build-info.json` 的同名值 —— 那正是本缺口「读错代」的病根）。
func artifactEmbeddedIdentity(p string) (code, built string) {
	code, built = buildEmbeddedNone, buildEmbeddedNone
	bi, err := buildinfo.ReadFile(p)
	if err != nil {
		return code, built
	}
	for _, s := range bi.Settings {
		if s.Key != "-tags" {
			continue
		}
		for _, part := range strings.Split(s.Value, ",") {
			kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
			if len(kv) != 2 {
				continue
			}
			switch kv[0] {
			case "commit":
				code = kv[1]
			case "build_time":
				built = kv[1]
			}
		}
	}
	return code, built
}

// buildOnlyOpen —— 本版**唯一开放执行**的 `--only` 档：`cli`（只写 `bin/zerg` 一件）。
//
// 为什么只开这一档（窄口子，不是把闸拆了）：`zerg build all --only cli` 的件是**命令面自己的二进制**
// —— 不常驻、不绑端口、不在跑（`build-all.sh:34-35` 逐字：本档「只写 `bin/zerg` 一个文件，
// 其余制品与本脚本写的 `build-info.json` **一个字节都不动** ✓」）⇒ 它不是换件，是**自举**
// （改命令树要能重编命令面，否则「只走命令面」这条铁律在**改命令面自己**时会当场死锁）。
// `core`（覆盖**正在跑**的主控件）与 `compat` 仍是不可逆档 ⇒ 照旧**未开放**，要跑请 Mr2109 拍。
const buildOnlyOpen = "cli"

// buildOnlyLevels —— `--only` 的档闭集（**唯一真源**：用法串、错误文案、执行面都读它）。
// 档名逐字对齐 `scripts/build/build-all.sh` 的**四枚**旗标（`--only-cli` / `--only-core` / `--only-compat` / `--only-agentd`）。
// ★ 2026-09-24（待拍清单终版 条 8 · `Q-206`）：`agentd` **收进闭集** —— 依据 = 脚本已正式有 `--only-agentd`
//
//	（批六① · 主仓 `e6d9830f`，逐字「子端换件专用档」），而命令面闭集只有三档 ⇒ **两侧名单不同源**
//	正是本条病根。档数（下面文案里的「四」）与闭集**同处一处**改，别只改一半。
var buildOnlyLevels = []string{"cli", "core", "compat", "agentd"}

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
		fmt.Fprintf(stderr, "%s: `--only` 只认四档：%s（给了 %q）\n", progName, strings.Join(buildOnlyLevels, "|"), only)
		fmt.Fprintf(stderr, "档名逐字对齐构建脚本：--only-cli / --only-core / --only-compat / --only-agentd\n")
		return exitUsage
	}
	script, effect := "scripts/build/build-all.sh", "重编**全部制品**（含正在跑的 zerg-core / zerg-agentd）并重签"
	if action == "release" {
		script, effect = prScriptRel, "打包发布件（布局三件：dist 目录 + 校验 + 清单）"
	}
	scriptArgs := []string{}
	openForExec := false
	if action == "all" && only == buildOnlyOpen {
		scriptArgs = []string{"--only-" + buildOnlyOpen}
		openForExec = true
		effect = "只写 `bin/zerg` 一件（命令面自举档）—— 其余制品与 `build-info.json` 一个字节不动"
	}
	// ★ 2026-09-24（待拍清单终版 条 8）：`agentd` 收进闭集后，**执行面同批随动** —— 计划件给**逐字脚本档**
	//   `--only-agentd`（子端换件专用档 · 批六①），档说明也按该档写；但**真跑仍未开放**（见下 `openForExec`）：
	//   `agentd` 覆盖的是**在跑**的子端制品 ⇒ 与 `core` / `compat` 同一档口径（换件档 ⇒ 要真跑请 Mr2109 拍）。
	//   本单只落到「**命令面读得到 + 计划件两态正确**」为止，不自行开放一条新的不可逆执行路径。
	if action == "all" && only == "agentd" {
		scriptArgs = []string{"--only-agentd"}
		effect = "只写 `bin/zerg-agentd` 一件（**子端换件专用档** · 批六①）—— 其余制品与 `build-info.json` 一个字节不动"
	}
	// ★ 缺口 `Q-226`（2026-09-26 · ★★/P1 · 决策单第 7 条「**单开一档**」）：`build release --manifest-only`
	//   = **清单档**，只重写 `dist/<版本>/release/` 的清单与目录内逐件 sha256。与换件档**解耦**：
	//   · 不编译（不调 `build-all.sh`）· 不交叉编 linux 三件 · 不 `rm -rf` 重打包 · 不编茧壁；
	//   · **不拷进 `bin/`、不重签、不重启** ⇒ 碰不到正在跑的件（爆炸半径 = `dist/` 一个目录）。
	//   为什么必须有这一档：闸⑥（清单新鲜度 · 公开制品档 = **阻断**）读 `manifest.json` 的
	//   `source_sha` 与 `dirty`；清单过期 ⇒ 预检判红、发布链无路可走 —— 而此前 CLI 里**没有**这条路
	//   （唯一相关命令 `build release` 是换件档 ⇒ 真跑拒执退 2）⇒ 只能手搓 `pack-release.sh`（违「CLI 为主」）。
	//   档名与语义照设计稿 `设计-发布清单重生成档-v1.0-20260925.md`（§3 推荐「加子档、不新开命令」）。
	if action == "release" && inv.manifestOnly {
		scriptArgs = []string{"--manifest-only"}
		openForExec = true
		effect = "只重写 `dist/<版本>/release/` 的**清单与逐件 sha256** —— 不编译、不重打包、不拷进 `bin/`、不重签、不重启（碰不到正在跑的件）"
	}
	// ★ 2026-10-10（Mr2109 拍板「开 build release 换件档」）：换件档真跑本版**开放**。
	//   为什么开：它是发布链的**唯一出口** —— `dist/<版本>/release/` 成品包只有这一条正门能出，
	//   而 publish preflight 的闸④/⑥ 判的就是那个目录（判不了 ⇒ 不给结论 ⇒ 发布链无路可走）。
	//   风险面 = 覆盖 `bin/` 里的在跑制品（属既定「换件直换、重启主控不必问」的授权面）。
	//   ✗ 只开 `release` 这一条：`build all`（默认全套）/`--only core`/`--only compat` 仍一律拒执。
	if action == "release" && !inv.manifestOnly {
		openForExec = true
		effect = "打包发布件（布局三件：dist 目录 + 校验 + 清单）—— 内含全量构建，会覆盖 `bin/` 里的在跑制品"
	}
	if inv.dryRun {
		fmt.Fprintln(stdout, "计划件（--dry-run · 零副作用 —— 未执行、未改任何状态）")
		fmt.Fprintf(stdout, "  动作     : %s build %s%s\n", progName, action, onlyNote(only))
		// 缺口 `Q-226`：清单档的计划件**逐条明写**「会跑哪几步 / 会写哪些件」——
		//   本档会写 `publish/` 外的落点（`dist/<版本>/release/` 三处）⇒ 必须在计划件里点名 ✗ 不许静默重打包。
		if action == "release" && inv.manifestOnly {
			fmt.Fprintf(stdout, "  危险档   : D2（清单档：只写 `dist/<版本>/release/` 一个目录 —— **不碰 `bin/` 里的在跑件**）\n")
			fmt.Fprintf(stdout, "  它会调   : %s\n", buildCallShow(script, scriptArgs))
			fmt.Fprintf(stdout, "  它会动   : %s\n", effect)
			fmt.Fprintf(stdout, "  会跑哪几步:\n")
			fmt.Fprintf(stdout, "    ① 前置检查 `dist/<版本>/release/` 存在且内有 zerg-* 制品（不在 ⇒ 退 8 · 不给结论）\n")
			fmt.Fprintf(stdout, "    ② 逐件重算 `dist/<版本>/release/<件>.sha256` 与 `checksums.txt`\n")
			fmt.Fprintf(stdout, "    ③ ZERG_MANIFEST_LOCAL=1 python3 scripts/evals/make-manifest.py <release 目录> <版本> <commit> <build_time>\n")
			fmt.Fprintf(stdout, "       （版本 / commit / build_time 取**既有清单**，退而取 `build-info.json` —— 保住「这批制品是谁产的」）\n")
			fmt.Fprintf(stdout, "    ④ 复跑脚本自带校验段：manifest 逐件 sha256 / 体积必须与文件一致\n")
			fmt.Fprintf(stdout, "  会写哪些件（**全在 `publish/` 外** —— 只此三处）:\n")
			fmt.Fprintf(stdout, "    dist/<版本>/release/manifest.json\n")
			fmt.Fprintf(stdout, "    dist/<版本>/release/<逐件>.sha256\n")
			fmt.Fprintf(stdout, "    dist/<版本>/release/checksums.txt\n")
			fmt.Fprintf(stdout, "  不会动   : `bin/`（在跑的件）· `wall/target/` · 不重编译 · 不拷贝二进制 · 不重签 · 不重启\n")
			fmt.Fprintf(stdout, "  执行要   : --confirm=<主机名> 与 --yes 同时到\n")
			fmt.Fprintf(stdout, "  本版状态 : **已开放**（Mr2109 2026-09-26 决策单第 7 条「单开一档」）\n")
			fmt.Fprintf(stdout, "  下一步   : %s publish preflight <产物目录>（只读复核 · 闸⑥ 应转绿）\n", progName)
			fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未执行、未改任何状态）")
			return exitOK
		}
		fmt.Fprintf(stdout, "  危险档   : D3（换件档：会覆盖 `bin/` 里的在跑制品）\n")
		fmt.Fprintf(stdout, "  它会调   : %s\n", buildCallShow(script, scriptArgs))
		fmt.Fprintf(stdout, "  它会动   : %s\n", effect)
		fmt.Fprintf(stdout, "  执行要   : --confirm=<主机名> 与 --yes 同时到\n")
		if openForExec && action == "release" {
			fmt.Fprintf(stdout, "  本版状态 : **已开放**（Mr2109 2026-10-10 拍板「开 build release 换件档」—— 发布链唯一出口）\n")
			fmt.Fprintf(stdout, "  下一步   : %s publish preflight dist/<版本>/release（只读复核 · 闸④/⑥ 应转绿）\n", progName)
		} else if openForExec {
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
	// ② 真跑（自举档 / 清单档）：D 档三态 —— `--confirm=<主机名>`（值必须与主机名逐字相同）与 `--yes` 同时到。
	//   缺口 `Q-226`：清单档写的是 `dist/`（**不碰 `bin/` 里的在跑件**）⇒ 与写 `bin/` 的自举档**风险档不同**，
	//   判词与退码同表（`2`），但错话不许把两件事说成一件 ⇒ 两档各给逐字判词。
	host := planHost()
	gateLevel, gateSubject, gateHint := "D3", fmt.Sprintf("`build all --only %s` 会写 `bin/`", buildOnlyOpen),
		fmt.Sprintf("%s build all --only %s --dry-run", progName, buildOnlyOpen)
	if action == "release" && inv.manifestOnly {
		gateLevel = "D2"
		gateSubject = "`build release --manifest-only` 会写 `dist/<版本>/release/`（**不碰 `bin/` 里的在跑件**）"
		gateHint = fmt.Sprintf("%s build release --manifest-only --dry-run", progName)
	}
	if action == "release" && !inv.manifestOnly {
		gateLevel = "D3"
		gateSubject = "`build release` 换件档会**全量构建并覆盖 `bin/` 里的在跑制品**，再打包 `dist/<版本>/release/`"
		gateHint = fmt.Sprintf("%s build release --dry-run", progName)
	}
	if !inv.confirmGiven {
		msg := fmt.Sprintf("%s ⇒ 按 %s 档：**缺确认 ⇒ 不执行**", gateSubject, gateLevel)
		inv.setErr("usage", "confirm_required", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		fmt.Fprintf(stderr, "要执行得给：--confirm=%s --yes\n", host)
		fmt.Fprintf(stderr, "先看计划件：%s\n", gateHint)
		return exitUsage
	}
	if !devEditHostAccept(inv.confirm) {
		msg := fmt.Sprintf("确认值不匹配目标（--confirm 给的是 %q，本机主机名是 %q）⇒ 不执行", inv.confirm, host)
		inv.setErr("usage", "confirm_mismatch", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		fmt.Fprintf(stderr, "`--confirm` 的值必须与目标**逐字相同**（§4.1 K7）—— 给错值一律拒绝，不许「当没给」\n")
		return exitUsage
	}
	if !inv.yes {
		msg := fmt.Sprintf("%s 是 %s 档 —— **缺 --yes ⇒ 不执行**", gateSubject, gateLevel)
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
	// ③ 执行面**真身**前置闸（fail-closed 保留 —— 只是把「件必须在盘」换成「能力可用」）。
	//   ① `build all` 的真跑执行面早改走**源码在仓的 Go 构建器**（下 ④：`cd scripts/build && go run ./cmd/zerg-build-all …`）
	//   ⇒ 闸看的**真身** = 「Go 工具链可用 ＋ `scripts/build/cmd/zerg-build-all/main.go` 在盘」；
	//   ✗ 不再是壳脚本 `scripts/build/build-all.sh` 在盘 —— 它一退役，真命令毫无影响、此闸却先假红（绑错了对象）。
	//   ② `build release` 已于**批B1 真搬**（`core/cmd/zerg/pack_release.go` · 2026-10-08）⇒ 执行真身是
	//      进程内 Go，✗ 不再是 `scripts/build/pack-release.sh`；保留的 fail-closed 面 = 它仍委派的
	//      **外部真身** `scripts/evals/make-manifest.py`（清单生成器 · 别件）在盘。
	if err := buildExecPrereq(root, script); err != nil {
		inv.setErr("blocked", "exec_prereq_absent", err.Error())
		fmt.Fprintf(stderr, "%s: %s ⇒ 不给结论（退码 8）\n", progName, err.Error())
		return exitBlocked
	}
	execNote := "只写 bin/zerg 一件"
	if action == "release" && inv.manifestOnly {
		execNote = "只写 `dist/<版本>/release/` 的清单与逐件 sha256 —— 不碰 `bin/` 里的在跑件"
	}
	// ★ 批-6 第三步（调用点接线 · 决-08 旧环护栏）：`build all` 的正门指到**源码在仓的 Go 构建器**
	//   （现编现跑 `cd scripts/build && go run ./cmd/zerg-build-all …`）—— 环内引导器 = Go 工具链 ＋
	//   仓内构建器源码，✗ 不 exec 刚产出的 `bin/*` ⇒ 「改命令面 ⇒ 编不出命令面」这条死锁当场解开。
	//   ★ 批B1（2026-10-08）：`build release` 已**真搬**（`core/cmd/zerg/pack_release.go`）⇒ 进程内
	//   直调，✗ 不再 `bash scripts/build/pack-release.sh …`（壳转账 → 真搬）。
	fmt.Fprintf(stderr, "%s: %s执行 —— %s（%s）\n", progName, gateLevel, buildCallShow(script, scriptArgs), execNote)
	if script == prScriptRel {
		// 退码**原样直通**（与旧壳同一口径 · §4.3 U1 · ✗ 不翻译）。
		return packReleaseRun(root, stdout, stderr, scriptArgs)
	}
	cmdName, cmdDir := "go", filepath.Join(root, "scripts", "build")
	cmdArgs := append([]string{"run", "./cmd/zerg-build-all"}, scriptArgs...)
	cmd := armChildGroup(exec.Command(cmdName, cmdArgs...))
	runningChild = cmd
	defer func() { runningChild = nil }()
	cmd.Dir = cmdDir
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

// buildCallShow —— 「正门 ↔ 构建器」接线（批-6 第三步）的**展示串**：与 cmdBuildPassthrough 真正
// exec 的形态**同源**（同一处判定），两处不许各写各的。
//
//	build all     ⇒ `cd scripts/build && go run ./cmd/zerg-build-all …`（源码在仓 · 现编现跑）
//	build release ⇒ **进程内 Go 真身** `core/cmd/zerg/pack_release.go`（批B1 真搬 · 2026-10-08）
//	                —— ✗ 不再是 `bash scripts/build/pack-release.sh …`（该壳已真搬）。
//
// ★ 旧环护栏（决-08）：展示串里出现的是**构建器源码路径**，✗ 不是刚产出的 `bin/*`。
func buildCallShow(script string, args []string) string {
	if script == prScriptRel {
		return "core/cmd/zerg/pack_release.go（进程内 Go 真身 · 真搬自 " + prScriptRel + "）" + prArgsSuffix(args)
	}
	return "cd scripts/build && go run ./cmd/zerg-build-all " + strings.Join(args, " ")
}

// buildExecPrereq —— 真跑档的**执行真身**可用性闸（fail-closed：不满足 ⇒ 出声 · 退码 8）。
//
// 与 `buildCallShow` 同源（都按同一个 `script` 判真身形态），✗ 不许两处各判各的：
//
//	`build release` ⇒ 执行面 = **进程内 Go 真身**（`core/cmd/zerg/pack_release.go` · 批B1 真搬）⇒
//	                  真身恒在；保留的 fail-closed 面 = 它仍要委派的**外部真身**
//	                  `scripts/evals/make-manifest.py`（清单生成器 · 别件）在盘。
//	`build all`     ⇒ 执行面 = **源码在仓的 Go 构建器**（`go run ./cmd/zerg-build-all`，
//	                  工作目录 `scripts/build`）⇒ 看**两样都可用**：
//	                  ① `go` 工具链在 PATH（能力可用）；② `scripts/build/cmd/zerg-build-all/main.go` 在盘（真身在）。
//
// ★ 2026-10-08（退役 build 簇）：✗ 不再看 `scripts/build/build-all.sh` —— `build all` 的执行面
// 早已不是它（见 `cmdBuildPassthrough` ④，走 `go run ./cmd/zerg-build-all`）。该壳脚本一退役，
// 真命令照跑，此闸若仍盯它就是**假红**；故换看「能力可用」（同旧语义的 fail-closed）。
// ★ 2026-10-08（批B1）：同理 ✗ 不再看 `scripts/build/pack-release.sh` —— 它已被真搬，脚本离盘
// 真命令照跑，再看它就是假红。
func buildExecPrereq(root, script string) error {
	if script == prScriptRel {
		if _, err := os.Stat(filepath.Join(root, prMakeManifestRel)); err != nil {
			return fmt.Errorf("发布清单生成器不在（%s ⇒ 清单档/组装档跑不动）", prMakeManifestRel)
		}
		return nil
	}
	if _, err := exec.LookPath("go"); err != nil {
		return fmt.Errorf("Go 工具链不可用（go 不在 PATH ⇒ 跑不动 `go run ./cmd/zerg-build-all`）")
	}
	if _, err := os.Stat(filepath.Join(root, "scripts", "build", "cmd", "zerg-build-all", "main.go")); err != nil {
		return fmt.Errorf("构建器真身不在（scripts/build/cmd/zerg-build-all/main.go）")
	}
	return nil
}
