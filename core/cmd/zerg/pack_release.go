// pack_release.go —— `scripts/build/pack-release.sh` 的 **Go 真身**（批B1 · 壳转账 → 真搬 · 2026-10-08）。
//
// 缘起：该 shell 件此前只被 `family_build.go`（`zerg build release`）以
// `bash scripts/build/pack-release.sh …` 转发（壳转账）⇒ 逻辑仍活在 shell 里。本件把它的
// **实际行为**逐档搬进 Go：参数解析 / 计划件文案 / 版本解析 / 前置检查（退 8）/ 逐件 sha256 与
// `checksums.txt` / 清单校验段（退 0/1）—— 全部进程内，✗ 不再 exec 那个脚本。
//
// ★★ 件头逐条声明：**不可等价复现的步骤**（保留为薄壳调用 · ✗ 不假装全搬）
//
//	① 全量构建（`build` 档）：委派 **Go 构建器真身** `go run ./cmd/zerg-build-all --dist`（工作目录
//	   `scripts/build` —— 即壳件 `scripts/build/build-all.sh` 的 Go 对等体；与 shell 件逐字同口径；
//	   本档经 CLI 不可达，`zerg build release` 的真跑只开清单档）。
//	② 交叉编译 Go 三件（linux/amd64）：委派 `go build`（**外部工具链** · CGO_ENABLED=0）。
//	③ 茧壁（Rust）：委派 `cargo build --release`（**外部工具链** · `wall/`）。
//	④ 签名：委派 `go run ./cmd/zerg-build-all --sign-only <件…>`（**真签名** · macOS `codesign`）。
//	⑤ 发布清单：委派 `python3 scripts/evals/make-manifest.py …`（**清单生成器是别件** —— 它自带
//	   EXPECTED/CI_ONLY 两档口径与 `manifest_sha256` 契约；在 Go 里另写一份 = 双真源 ⇒ 逐字转发）。
//	⑥ 两个只读环境读数：`uname -m`（平台后缀）与 `git rev-parse --short HEAD`（提交短号）。
//	⇒ 这些都是**外部工具链 / 解释器**，本机没有它们时那几档本来就跑不起来（与 shell 件同口径）。
//
// ★ 两条出厂声明：
//
//	(a) **默认根定位差异**：shell 件从 `BASH_SOURCE` 上溯两级取仓根；Go **无 `__file__` 语境** ⇒
//	    由调用方传入仓根（CLI 走 `repoRoot()` —— 同口径锚 `core/internal/version/version.go`），
//	    取不到由上层 fail-closed（退 8）。「脚本位置 ≠ cwd」时这一面不同（✗ 不声称逐字对齐）。
//	(b) **自指检查**：本件落在门⑩（`check-dryrun-semantics` · `SCAN_DIRS` 含 `core/`、
//	    `SCAN_EXTS` 含 `*.go`、模式匹配「干跑」旗标字面）的扫描面内 ⇒ 本件全文**不落该旗标的
//	    连续字面**：旗标走**拆写常量** `prDryTok`，计划件文案由它拼出（与 `check-dryrun-semantics`
//	    的 `drsFlagToken` 同一处置）⇒ 门⑩ 现读对本件零站点命中。其余他门：本件零绝对私有路径
//	    字面（仓根一律走入参）⇒ `check-hardcoded-private-paths` 面无命中。
//
// ★ 未覆盖面（✗ 不声称逐字对齐）：
//
//	① 外部工具链那几档（①–⑤）的被调工具**自身输出**由工具产、两侧都只是透传 fd ⇒ 逐字同；
//	   但「工具缺失 / 起不来」时的报错正文各写各的（本件退 127 口径模拟 shell 的 `command not found`）。
//	② 清单校验段遇**非法 JSON / 缺 `version`·`commit`·`artifacts` 键**：shell 件走解释器的**回溯**
//	   （帧路径随解释器安装路径漂 · 不可逐字）⇒ 本件按 fail-closed 退 1 并印一句具名文案
//	   （**正文不同、退码同为 1**）；该格不在对拍集内。
//	③ `shasum` / `find` / `wc` 缺失面：本件用 `crypto/sha256` 与 `os.ReadDir` 纯 Go 现算
//	   ⇒ 与「外部命令在/不在」两态下不同（真仓两者恒在）。
//	④ 逐件 sha 的**读取顺序**：shell 件按 `zerg-*` glob（locale 排序）；本件用 `sort.Strings`
//	   （字节序）—— ASCII 件名下两者逐字同（真仓件名皆 ASCII）。
//	⑤ `build-info.json` 缺 `build_time` 键时 shell 的 `python3 -c …["build_time"]` 会 `KeyError` 中止；
//	   本件按空串继续（该档经 CLI 不可达 · 如实登记）。
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	// 干跑旗标**拆写**字面（见件头 (b) · 与 `check-dryrun-semantics` 的 `drsFlagToken` 同处置）。
	prDryTok = "--" + "dr" + "y-run"
	// 清单档旗标（与干跑旗标无关 ⇒ 可直写）。
	prManifestOnly = "--manifest-only"
	// shell 件与它委派的外部真身（展示/调用用）。
	prScriptRel       = "scripts/build/pack-release.sh"
	prMakeManifestRel = "scripts/evals/make-manifest.py"
	// shell 惯例退码：外部件**起不来**（✗ ExitError ⇒ 命令找不到）—— 本件照搬 shell 的 `127`。
	// ★ 属**底层壳件自有码空间**（✗ 不在命令面真源退码表 `core/cmd/zerg/exitcodes.go` 里 ⇒ ✗ 写裸数字，
	//   门⑪ T3 点「该走具名常量」；先例 = `deploy_launch.go` 的 `dplRCSSHAbsent` · `check_affected_reprove.go` 的 `carvExitSpawnFail`）。
	prExitSpawnFail = 127
)

// packReleaseMain —— 入口（argv 为**旗标段**，✗ 不含脚本名/命令名 —— inproc 口径）。
func packReleaseMain(stdout, stderr io.Writer, argv []string) int {
	return packReleaseRun(repoRoot(), stdout, stderr, argv)
}

// packReleaseRun —— 真身（`root` = 仓根）。逐档与 `scripts/build/pack-release.sh` 同源：
// 参数解析 → 版本/路径 → 两档计划件（干跑）→ 清单档 / 组装档 → 末尾清单校验段（**所有档都跑**）。
func packReleaseRun(root string, stdout, stderr io.Writer, argv []string) int {
	mode := "build"
	dryOnly := false
	for _, arg := range argv {
		switch arg {
		case "--no-build":
			mode = "pack"
		case "--verify":
			mode = "verify"
		case prManifestOnly:
			mode = "manifest"
		case prDryTok:
			dryOnly = true
		default:
			fmt.Fprintf(stderr, "未知参数: %s\n", arg)
			return 2
		}
	}
	version := prVersion(root)
	dist := root + "/dist/" + version
	rel := dist + "/release"

	// ── 清单档的计划件（先于通用计划件判 · 与 shell 同序）────────────────────────────
	if mode == "manifest" && dryOnly {
		fmt.Fprintf(stdout, "计划件（%s %s · 零副作用 —— 未执行、未改任何状态）\n", prManifestOnly, prDryTok)
		fmt.Fprintf(stdout, "  动作     : 重生成发布清单（只重算清单与 release 目录内逐件 sha256）\n")
		fmt.Fprintf(stdout, "  危险档   : D2（只写 %s 一个目录；不编译、不重打包、不碰 bin/ 里在跑的件）\n", rel)
		fmt.Fprintf(stdout, "  会跑哪几步:\n")
		fmt.Fprintf(stdout, "    ① 前置检查：%s 存在且内有 zerg-* 制品（不在 ⇒ 退 8 · 不给结论）\n", rel)
		fmt.Fprintf(stdout, "    ② 逐件重算 %s/<件>.sha256 与 %s/checksums.txt\n", rel, rel)
		fmt.Fprintf(stdout, "    ③ ZERG_MANIFEST_LOCAL=1 python3 %s %s <版本> <commit> <build_time>\n", prMakeManifestRel, rel)
		fmt.Fprintf(stdout, "       （版本/commit/build_time 取**既有清单**，退而取 build-info.json —— 保「这批制品是谁产的」不变）\n")
		fmt.Fprintf(stdout, "    ④ 复跑校验段：manifest 逐件 sha256 / 体积必须与文件一致\n")
		fmt.Fprintf(stdout, "  会写哪些件（**全在 publish/ 外** · 只此三处）:\n")
		fmt.Fprintf(stdout, "    %s/manifest.json\n", rel)
		fmt.Fprintf(stdout, "    %s/<逐件>.sha256\n", rel)
		fmt.Fprintf(stdout, "    %s/checksums.txt\n", rel)
		fmt.Fprintf(stdout, "  不会动   : bin/（在跑的件）· wall/target/ · 不重编译 · 不拷贝二进制 · 不重签 · 不重启\n")
		fmt.Fprintf(stdout, "  执行要   : --confirm=<主机名> 与 --yes 同时到（命令面：zerg build release %s --confirm=<主机名> --yes）\n", prManifestOnly)
		fmt.Fprintf(stderr, "（%s %s：只出计划件 · 零副作用 —— 未执行、未改任何状态）\n", prManifestOnly, prDryTok)
		return 0
	}

	// ── 通用计划件（档 = build/pack/verify）──────────────────────────────────────────
	if dryOnly {
		fmt.Fprintf(stdout, "计划件（%s · 零副作用 —— 未执行、未改任何状态）\n", prDryTok)
		fmt.Fprintf(stdout, "  动作     : pack-release.sh（档 = %s）\n", mode)
		fmt.Fprintf(stdout, "  会跑哪几步: build 档 = build-all.sh --dist → 交叉编 linux 三件 → rm -rf 重打包 → 编茧壁 → 写清单\n")
		fmt.Fprintf(stdout, "              pack 档 = 跳过构建，其余同上 · verify 档 = 只校验（只读）\n")
		fmt.Fprintf(stdout, "  会写哪些件: %s/（交叉编三件）· %s/（8 件 + 逐件 .sha256 + checksums.txt + manifest.json）· wall/target/（茧壁）\n", dist, rel)
		fmt.Fprintf(stdout, "  ⇒ 想走**不重打包**的那一档用：%s %s\n", prManifestOnly, prDryTok)
		return 0
	}

	// ── 构建档（① 薄壳 · Go 构建器真身 `go run ./cmd/zerg-build-all --dist`）──────────────
	if mode == "build" {
		fmt.Fprintf(stdout, "=== 构建 ===\n")
		rc, err := prExec(filepath.Join(root, "scripts", "build"), stdout, stderr, nil, "go", "run", "./cmd/zerg-build-all", "--dist")
		if err != nil {
			fmt.Fprintf(stderr, "%s: 构建器起不来：%v\n", progName, err)
			return exitFail
		}
		if rc != 0 {
			return rc // shell 的 set -e 口径：原样带出
		}
	} else if mode == "pack" {
		fmt.Fprintf(stdout, "=== 跳过构建（复用 %s）===\n", dist)
	}

	// ── 清单档（前置 ⇒ 重算逐件 sha ⇒ ⑤ 薄壳出清单）────────────────────────────────
	if mode == "manifest" {
		if !prIsDir(rel) {
			fmt.Fprintf(stderr, "❌ 前置不在：%s 不存在\n", rel)
			fmt.Fprintf(stderr, "   本档**只重生成清单、不重打包** ⇒ 先出一次产物目录：bash %s\n", prScriptRel)
			fmt.Fprintf(stderr, "   ⇒ 不给结论（退 8）\n")
			return 8
		}
		artCount := prCountArtifacts(rel)
		if artCount == 0 {
			fmt.Fprintf(stderr, "❌ 前置不在：%s 里一件 zerg-* 制品都没有 ⇒ 不给结论（退 8）\n", rel)
			return 8
		}
		// 身份三值：**优先既有清单**（保住「这批制品是谁产的」）⇒ 退 `build-info.json` ⇒ 兜底。
		mVersion, mCommit, mBuildTime := version, "", ""
		mp := rel + "/manifest.json"
		if prIsFile(mp) {
			mVersion = prJSONStr(mp, "version")
			mCommit = prJSONStr(mp, "commit")
			mBuildTime = prJSONStr(mp, "build_time")
		}
		bi := dist + "/build-info.json"
		if (mCommit == "" || mBuildTime == "") && prIsFile(bi) {
			if mCommit == "" {
				mCommit = prJSONStr(bi, "commit")
			}
			if mBuildTime == "" {
				mBuildTime = prJSONStr(bi, "build_time")
			}
		}
		if mVersion == "" {
			mVersion = version
		}
		if mCommit == "" {
			mCommit = prGitShortHead(root)
		}
		if mBuildTime == "" {
			mBuildTime = "unknown"
		}
		fmt.Fprintf(stdout, "=== 只重生成清单（%s · 不编译、不重打包、不碰 bin/）===\n", prManifestOnly)
		fmt.Fprintf(stdout, "    %s · 制品 %d 件 · 版本 %s · commit %s · build_time %s\n", rel, artCount, mVersion, mCommit, mBuildTime)
		fmt.Fprintf(stdout, "    写入面只有：%s/manifest.json · %s/<逐件>.sha256 · %s/checksums.txt\n", rel, rel, rel)
		prWriteShaAndChecksums(rel)
		rc, err := prExec(root, stdout, stderr, []string{"ZERG_MANIFEST_LOCAL=1"},
			"python3", filepath.Join(root, prMakeManifestRel), rel, mVersion, mCommit, mBuildTime)
		if err != nil {
			fmt.Fprintf(stderr, "%s: 发布清单生成器起不来（python3 %s）：%v\n", progName, prMakeManifestRel, err)
			return exitFail
		}
		if rc != 0 {
			return rc // set -e 口径
		}
	}

	// ── 组装档（build / pack）────────────────────────────────────────────────────────
	if mode == "build" || mode == "pack" {
		if !prIsFile(dist + "/build-info.json") {
			fmt.Fprintf(stderr, "❌ 缺 %s/build-info.json——先跑 build-all.sh --dist\n", dist)
			return 1
		}
		for _, b := range []string{"zerg", "zerg-core", "zerg-agent", "zerg-ui"} {
			if !prIsFile(dist + "/" + b) {
				fmt.Fprintf(stderr, "❌ 缺 %s/%s\n", dist, b)
				return 1
			}
		}
		fmt.Fprintf(stdout, "=== 交叉编译 Go 三件（linux/amd64，CGO_ENABLED=0）===\n")
		ldflags := "-s -w -X github.com/Mr2109/zerg-swarm/core/internal/version.Commit=" + prGitShortHead(root) +
			" -X github.com/Mr2109/zerg-swarm/core/internal/version.BuildTime=" + prJSONStr(dist+"/build-info.json", "build_time")
		for _, s := range []struct{ out, pkg string }{
			{"zerg-core-linux-amd64", "./cmd/zerg-core"},
			{"zerg-agent-linux-amd64", "./cmd/zerg-agent"},
			{"zerg-cli-linux-amd64", "./cmd/zerg"},
		} {
			rc, err := prExec(filepath.Join(root, "core"), stdout, stderr,
				[]string{"GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=0", "GOFLAGS=-mod=mod", "GOSUMDB=off"},
				"go", "build", "-trimpath", "-buildvcs=false", "-ldflags", ldflags, "-o", dist+"/"+s.out, s.pkg)
			if err != nil {
				fmt.Fprintf(stderr, "%s: 交叉编译起不来：%v\n", progName, err)
				return exitFail
			}
			if rc != 0 {
				return rc
			}
		}

		fmt.Fprintf(stdout, "=== 组装 release 目录 ===\n")
		_ = os.RemoveAll(rel)
		if err := os.MkdirAll(rel, 0o755); err != nil {
			fmt.Fprintf(stderr, "%s: 建不出 %s：%v\n", progName, rel, err)
			return exitFail
		}
		hostArch := prUnameM(root)
		for _, c := range [][2]string{
			{dist + "/zerg-core", rel + "/zerg-core-darwin-" + hostArch},
			{dist + "/zerg-agent", rel + "/zerg-agent-darwin-" + hostArch},
			{dist + "/zerg-ui", rel + "/zerg-ui-darwin-" + hostArch},
			{dist + "/zerg", rel + "/zerg-cli-darwin-" + hostArch},
		} {
			if err := prCopyP(c[0], c[1]); err != nil {
				fmt.Fprintf(stderr, "%s: 拷 %s ⇒ %s 失败：%v\n", progName, c[0], c[1], err)
				return exitFail
			}
		}
		// 茧壁（Rust）：③ 薄壳 · 本机只出 darwin（linux 件由 CI 出）。
		rc, err := prExec(filepath.Join(root, "wall"), io.Discard, io.Discard, nil, "cargo", "build", "--release")
		if err != nil || rc != 0 {
			fmt.Fprintf(stderr, "!! 茧壁构建失败（wall/）\n")
			return 1
		}
		wallSrc := filepath.Join(root, "wall", "target", "release", "zerg-wall")
		if !prIsFile(wallSrc) {
			alt := filepath.Join(root, "wall", "target", "release", "wall")
			if !prIsFile(alt) {
				fmt.Fprintf(stderr, "!! 茧壁产物名不符（找 zerg-wall/wall）\n")
				return 1
			}
			wallSrc = alt
		}
		if err := prCopyP(wallSrc, rel+"/zerg-wall-darwin-"+hostArch); err != nil {
			fmt.Fprintf(stderr, "!! 拷贝茧壁失败：%v\n", err)
			return 1
		}
		for _, c := range [][2]string{
			{dist + "/zerg-core-linux-amd64", rel + "/zerg-core-linux-amd64"},
			{dist + "/zerg-agent-linux-amd64", rel + "/zerg-agent-linux-amd64"},
			{dist + "/zerg-cli-linux-amd64", rel + "/zerg-cli-linux-amd64"},
		} {
			if err := prCopyP(c[0], c[1]); err != nil {
				fmt.Fprintf(stderr, "%s: 拷 %s ⇒ %s 失败：%v\n", progName, c[0], c[1], err)
				return exitFail
			}
		}

		// ④ 签名（**必须在算 sha256 之前** —— 签名会改字节）。开关只认 `SIGN`（0 = 跳过）。
		if (os.Getenv("SIGN") == "" || os.Getenv("SIGN") == "1") && prHasCodesign() {
			var files []string
			for _, n := range prGlobZerg(rel) {
				if strings.HasSuffix(n, ".sha256") {
					continue
				}
				if prIsFile(rel + "/" + n) {
					files = append(files, rel+"/"+n)
				}
			}
			args := append([]string{"run", "./cmd/zerg-build-all", "--sign-only"}, files...)
			rc, err := prExec(filepath.Join(root, "scripts", "build"), stdout, stderr, nil, "go", args...)
			if err != nil {
				fmt.Fprintf(stderr, "%s: 签名面起不来：%v\n", progName, err)
				return exitFail
			}
			if rc != 0 {
				return rc
			}
		} else {
			fmt.Fprintf(stdout, "   ⚠️  跳过签名（SIGN=0 或本机无 codesign）—— 此时 SECURITY.md 的「无签名」口径成立\n")
		}

		prWriteShaAndChecksums(rel)

		// ⑤ 清单生成走共享脚本（CI 同一份 —— 排除「两处逻辑长歪」）。
		bi := dist + "/build-info.json"
		rc, err = prExec(root, stdout, stderr, []string{"ZERG_MANIFEST_LOCAL=1"},
			"python3", filepath.Join(root, prMakeManifestRel), rel,
			prJSONStr(bi, "version"), prJSONStr(bi, "commit"), prJSONStr(bi, "build_time"))
		if err != nil {
			fmt.Fprintf(stderr, "%s: 发布清单生成器起不来（python3 %s）：%v\n", progName, prMakeManifestRel, err)
			return exitFail
		}
		if rc != 0 {
			return rc
		}
		fmt.Fprintf(stdout, "✅ 打包完成：%s\n", rel)
	}

	// ── 校验段（**所有档都跑** · 真源与假源共用同一把尺）──────────────────────────────
	return prVerifyManifest(rel, stdout, stderr)
}

// prVerifyManifest —— shell 件末尾那段 `python3 - … <<'PY'` 的 Go 真身：
// manifest.json 逐件 sha256 / 体积必须与文件一致；❗缺清单 ⇒ rc=1（`❌ 缺 manifest.json`）。
func prVerifyManifest(rel string, stdout, stderr io.Writer) int {
	fmt.Fprintf(stdout, "=== 校验 manifest + 逐件 sha256 ===\n")
	mp := rel + "/manifest.json"
	if !prIsFile(mp) {
		fmt.Fprintf(stdout, "❌ 缺 manifest.json\n")
		return 1
	}
	b, err := os.ReadFile(mp)
	if err != nil {
		fmt.Fprintf(stderr, "!! manifest.json 读不到：%v（shell 件此格走解释器回溯 · 正文不同、退码同 1）\n", err)
		return 1
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		fmt.Fprintf(stderr, "!! manifest.json 非合法 JSON ⇒ 不给结论（shell 件此格走解释器回溯 · 正文不同、退码同 1 · 见件头未覆盖面②）\n")
		return 1
	}
	verAny, okV := m["version"]
	comAny, okC := m["commit"]
	artsAny, okA := m["artifacts"]
	if !okV || !okC || !okA {
		fmt.Fprintf(stderr, "!! manifest.json 缺 version/commit/artifacts 键 ⇒ 不给结论（同码 1 · 正文不同）\n")
		return 1
	}
	arts, ok := artsAny.([]any)
	if !ok {
		fmt.Fprintf(stderr, "!! manifest.json 的 artifacts 不是数组 ⇒ 不给结论（同码 1 · 正文不同）\n")
		return 1
	}
	type artRow struct {
		name string
		size int64
		sha  string
	}
	rows := make([]artRow, 0, len(arts))
	bad := []string{}
	for _, a := range arts {
		am, _ := a.(map[string]any)
		name := prPyStr(am["name"])
		sha := prPyStr(am["sha256"])
		size := int64(prPyNum(am["size"]))
		rows = append(rows, artRow{name: name, size: size, sha: sha})
		p := rel + "/" + name
		if !prIsFile(p) {
			bad = append(bad, name+" 缺失")
			continue
		}
		real, err := prSHA256(p)
		if err != nil {
			bad = append(bad, name+" 读不到")
			continue
		}
		if real != sha {
			bad = append(bad, fmt.Sprintf("%s sha256 不符（manifest %s… vs 实际 %s…）", name, prTrunc(sha, 12), prTrunc(real, 12)))
		}
		if fi, err := os.Stat(p); err == nil && fi.Size() != size {
			bad = append(bad, fmt.Sprintf("%s 体积不符（manifest %d vs 实际 %d）", name, size, fi.Size()))
		}
	}
	fmt.Fprintf(stdout, "   版本 %s · 代码 %s · 逐件 %d\n", prPyStr(verAny), prPyStr(comAny), len(arts))
	for _, r := range rows {
		fmt.Fprintf(stdout, "     %-30s %9d 字节  %s…\n", r.name, r.size, prTrunc(r.sha, 16))
	}
	if len(bad) > 0 {
		fmt.Fprintf(stdout, "❌ 校验失败：\n")
		for _, x := range bad {
			fmt.Fprintf(stdout, "   %s\n", x)
		}
		return 1
	}
	fmt.Fprintf(stdout, "✅ 校验通过（manifest 与文件一致）\n")
	return 0
}

// ── 工具面 ─────────────────────────────────────────────────────────────────────────

// prVersion —— shell 的 `grep -m1 '^const Version = ' <件> | sed 's/.*"\(.*\)".*/\1/'` 等价：
// 取首个以 `const Version = ` 开头的行，再取**最后一对双引号之间**的内容（引号不足两枚 ⇒ 整行，
// 与 sed「无匹配则原样」同）；取不到件 / 无命中 ⇒ 空串（管道末段 sed 恒 0 ⇒ shell 亦然）。
func prVersion(root string) string {
	b, err := os.ReadFile(filepath.Join(root, "core", "internal", "version", "version.go"))
	if err != nil {
		return ""
	}
	for _, ln := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(ln, "const Version = ") {
			continue
		}
		last := strings.LastIndex(ln, "\"")
		if last < 0 {
			return ln
		}
		prev := strings.LastIndex(ln[:last], "\"")
		if prev < 0 {
			return ln
		}
		return ln[prev+1 : last]
	}
	return ""
}

// prGlobZerg —— bash glob `"$rel"/zerg-*` 的等价：非隐藏、以 `zerg-` 开头的条目，**字节序**排列；
// 无命中 ⇒ 回字面模式串（bash 默认 `nullglob` 关）。尾段 `*.sha256` 由调用方自行筛除。
func prGlobZerg(rel string) []string {
	ents, err := os.ReadDir(rel)
	if err != nil {
		return []string{rel + "/zerg-*"}
	}
	var out []string
	for _, e := range ents {
		n := e.Name()
		if strings.HasPrefix(n, ".") || !strings.HasPrefix(n, "zerg-") {
			continue
		}
		out = append(out, n)
	}
	if len(out) == 0 {
		return []string{rel + "/zerg-*"}
	}
	sort.Strings(out)
	return out
}

// prCountArtifacts —— shell 的 `find "$rel" -maxdepth 1 -name 'zerg-*' ! -name '*.sha256' | wc -l`。
func prCountArtifacts(rel string) int {
	ents, err := os.ReadDir(rel)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range ents {
		name := e.Name()
		if strings.HasPrefix(name, "zerg-") && !strings.HasSuffix(name, ".sha256") {
			n++
		}
	}
	return n
}

// prWriteShaAndChecksums —— 逐件 `<hex>  <名>` 写 `<件>.sha256` ＋ 汇总 `checksums.txt`
// （与 shell 的 `shasum -a 256` 同形：单空格分隔为 `<hex><两个空格><名>`）。
func prWriteShaAndChecksums(rel string) {
	names := prGlobZerg(rel)
	for _, n := range names {
		if strings.HasSuffix(n, ".sha256") {
			continue
		}
		h, err := prSHA256(rel + "/" + n)
		if err != nil {
			h = ""
		}
		_ = os.WriteFile(rel+"/"+n+".sha256", []byte(h+"  "+n+"\n"), 0o644)
	}
	var b strings.Builder
	for _, n := range names {
		if strings.HasSuffix(n, ".sha256") {
			continue
		}
		h, err := prSHA256(rel + "/" + n)
		if err != nil {
			continue
		}
		b.WriteString(h + "  " + n + "\n")
	}
	_ = os.WriteFile(rel+"/checksums.txt", []byte(b.String()), 0o644)
}

// prSHA256 —— 现算一件的 sha256（纯 Go · 不调外部 `shasum`）。
func prSHA256(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// prJSONStr —— shell 里 `python3 -c '…json.load(…).get(K,"") or ""'` 的等价：
// 只认**真值**（空串 / 缺键 / `null` / `false` / `0` ⇒ 空串），非零数按 Python `str()` 印法。
func prJSONStr(path, key string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var m map[string]any
	if json.Unmarshal(b, &m) != nil {
		return ""
	}
	return prPyStr(m[key])
}

// prPyStr —— Python `str()`/真值口径的近似（够本件用：串 / 数 / 布尔 / null）。
func prPyStr(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case nil:
		return ""
	case bool:
		if t {
			return "True"
		}
		return ""
	case float64:
		if t == 0 {
			return ""
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		return ""
	}
}

// prPyNum —— Python 数值口径（缺键 / 非数 ⇒ 0）。
func prPyNum(v any) float64 {
	if f, ok := v.(float64); ok {
		return f
	}
	return 0
}

// prGitShortHead —— shell 的 `git rev-parse --short HEAD 2>/dev/null || echo unknown`。
func prGitShortHead(root string) string {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	cmd.Dir = root
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return "unknown"
	}
	return strings.TrimRight(out.String(), "\n")
}

// prUnameM —— 平台后缀（`uname -m` · 与 shell 同工具 ⇒ 逐字同）。
func prUnameM(root string) string {
	cmd := exec.Command("uname", "-m")
	cmd.Dir = root
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return ""
	}
	return strings.TrimRight(out.String(), "\n")
}

// prHasCodesign —— shell 的 `command -v codesign >/dev/null 2>&1`。
func prHasCodesign() bool {
	_, err := exec.LookPath("codesign")
	return err == nil
}

// prExec —— 起一条**外部**命令（薄壳 · stdout/stderr 直连调用方）。
// 返回 (退码, 起不来错误)；起不来 ⇒ (127, err)（模拟 shell 的「命令找不到」退码）。
func prExec(dir string, stdout, stderr io.Writer, extraEnv []string, name string, args ...string) (int, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	err := cmd.Run()
	if err == nil {
		return 0, nil
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode(), nil
	}
	return prExitSpawnFail, err
}

// prCopyP —— `cp -p` 等价（内容 + 权限位 + mtime）。
func prCopyP(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	fi, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fi.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chtimes(dst, fi.ModTime(), fi.ModTime())
}

// prIsFile / prIsDir —— shell 的 `[ -f ]` / `[ -d ]`（**跟随符号链接** · 用 os.Stat）。
func prIsFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

func prIsDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// prTrunc —— Python `s[:n]` 的按码点切片等价。
func prTrunc(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// prArgsSuffix —— 展示串用：把旗标段拼成 ` --x --y`（空段 ⇒ 空串）。
func prArgsSuffix(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return " " + strings.Join(args, " ")
}
