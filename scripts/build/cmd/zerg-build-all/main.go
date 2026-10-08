// Command zerg-build-all — 虫族自研构建器（批-6 自举链 · **第二步**：`scripts/build/build-all.sh` 的 Go 化对等体）。
//
// 设计依据：`~/.zerg/state/设计-批6-自举链-谁编谁-20261007.md` §四（落地序）· §三（硬约束）。
// 拍板：决-06（同一独立 module `zerg-build` · ✗ 不进 core）· 决-07（签名面并入 Go 包 `internal/sign`）·
// 决-08（保留命令面自举环；环内引导器＝**已装** `bin/zerg` 或 Go 工具链，✗ 不依赖刚产出的新件）。
//
// 落点 / 形态（决-06）：独立 module = `scripts/build/go.mod`（试点已建）；现编现跑：
//
//	cd scripts/build && go run ./cmd/zerg-build-all [旗标]
//
// ✗ 不 import 任何被它构建的模块（core / agent / shared）⇒ 鸡生蛋深度 = 1。
//
// ★ 硬约束（一块不许丢 · 设计稿 §三）：
//
//	① 稳定身份签名：走 `internal/sign`（`lib-sign.sh` 的唯一 Go 对等体）—— stable/adhoc 两态；
//	   找不到身份回退 ad-hoc 并告警（与 .sh 逐字同）；✗ 不因搬而退回 adhoc。
//	② 身份双份注入：`-ldflags -X …version.{Commit,BuildTime}` ＋ `-tags commit=…,build_time=…`
//	   （后者进**嵌入段**，`go version -m` 可读 —— 因 GOFLAGS 带 `-trimpath` 时 ldflags 不进嵌入段，
//	   见 build-all.sh:152-161 / go.dev/issue/52372）。两条形态逐字保留。
//	③ 留档 `archive_prev_piece`：落点只许 `bin/_history/<件>.bak-<stamp>` · sha256 去重只留不删 ·
//	   `cp -p` 后回读 sha 校验 · 失败 ⇒ 拒换件。**调用点 2 处**：`bin/zerg-agentd`（--only-agentd）·
//	   `bin/zerg`（全量 / --only-cli · ✗ --dist 档不挂）。
//	④ 自净化：启动即对子进程剔除 `ZERG_STATE_DIR / ZERG_REPO / ZERG_PROPOSAL_DIR` ＋ TMPDIR 落仓外。
//
// ★ 沿用试点经验（②条血证）：仓根误判 —— `wall/` 与 `scripts/wall/` 同名 ⇒ 上溯 ＋ 双标记判定（见 zgb.IsRepoRoot）。
//
// 落地序（设计稿 §四 · 第二步）：
//
//	① 茧壁 `build-wall.sh`（试点 · 已完成）→ ② `build-all.sh`（本件）。
//	茧壁段本件**委派** **Go 真身** `cmd/zerg-build`（`build-wall.sh` 的 Go 化对等体 · 落 :338 那一行
//	现编现跑 `<仓>/scripts/build`）—— 该 .sh 按硬约束保留作两态基线，试点 Go 茧壁器已接线（2026-10-09）。
//
// 退出码（与 .sh 同口径）：0 成功 · 64 用法错 · 1 硬失败（读不到版本 / 自净化失败 / 留档失败 / 编不出）。
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

	"zerg-build/internal/sign"
	"zerg-build/internal/zgb"
)

const usage = `用法: go run ./cmd/zerg-build-all [旗标]（与 scripts/build/build-all.sh 同口径）
  --dist        产出到 dist/<版本>/（打包发布用；只写 dist/，一个字都不写 bin/、不重签 bin/）
  --no-ui       只建 Go 件（快）
  --public      UI 用 --no-default-features（对齐公开快照形态）
  --no-sign     跳过 codesign（非 macOS / 调试）
  --only-cli    只出命令面 bin/zerg（薄壳开发用）
  --only-core   只出主控 bin/zerg-core（换件专用）
  --only-compat 只出兼容层 bin/zerg-compat（回滚前置校验用）
  --only-agentd 只出子端 bin/zerg-agentd（子端换件专用）
  --repo-root <仓根>  覆盖仓根（默认由本源码编译期路径上溯推出）
  --help | -h   印本用法
环境: SOURCE_DATE_EPOCH（可选 · 业界可复现口径）冻结 BUILD_TIME；缺省 = 现时 UTC（与 .sh 逐字同）`

// buildInfo 的字段顺序与 `build-all.sh:465-472` 的 heredoc 逐字一致（version/commit/build_time/builder_host）。
type buildInfo struct {
	Version     string `json:"version"`
	Commit      string `json:"commit"`
	BuildTime   string `json:"build_time"`
	BuilderHost string `json:"builder_host"`
}

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

// runSignOnly —— `--sign-only <文件>…`：**只**走签名面（`internal/sign` 的唯一实现），
// 不编译、不打包、不碰进程。两个 shell 消费者（`build-all.sh` / `pack-release.sh`）用它替代
// 「`source lib-sign.sh` + resolve_sign_identity + sign_one」——两态（rc/stdout/stderr）与 shell 件逐字同。
//
// 与 `.sh` 逐字同口径（`lib-sign.sh:22-84`）：
//
//	· `SIGN`（环境变量 · 缺省 1）：`0` ⇒ 两个函数都直接返回（不签、不打印）；
//	· 无 `codesign` ⇒ 同样直接返回（静默）；身份解析走 `ZERG_SIGN_ID`（缺省 `Zerg Local Signing`）；
//	· 探针落点 = 环境变量 `TMPDIR`（缺省 `/tmp`）—— 与 shell 的 `${TMPDIR:-/tmp}` 同；
//	· 逐件 fail-open：`codesign` 非 0 不阻断（Linux ELF 一类签不上不算构建失败），退码恒 0。
func runSignOnly(files []string, stdout io.Writer) int {
	enabled := os.Getenv("SIGN") != "0"
	logf := func(format string, a ...any) { fmt.Fprintf(stdout, format+"\n", a...) }
	r := sign.Resolve(enabled, os.Getenv("TMPDIR"), logf)
	for _, f := range files {
		sign.One(r, f, logf)
	}
	return 0
}

func run(args []string, stdout, stderr io.Writer) int {
	// 签名面入口（构建模块的私有档 · ✗ 不进 CLI 命令树）：**最先判**，不吃下面的旗标表。
	if len(args) > 0 && args[0] == "--sign-only" {
		return runSignOnly(args[1:], stdout)
	}
	var (
		dist, noUI, public, noSign                bool
		onlyCLI, onlyCore, onlyCompat, onlyAgentd bool
		repoRootOverride                          string
	)
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dist":
			dist = true
		case "--no-ui":
			noUI = true
		case "--public":
			public = true
		case "--no-sign":
			noSign = true
		case "--only-cli":
			onlyCLI = true
		case "--only-core":
			onlyCore = true
		case "--only-compat":
			onlyCompat = true
		case "--only-agentd":
			onlyAgentd = true
		case "--repo-root":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "❌ --repo-root 缺路径参数（本件不静默忽略）")
				return 2
			}
			i++
			repoRootOverride = args[i]
		case "--help", "-h":
			fmt.Fprintln(stdout, usage)
			return 0
		default:
			fmt.Fprintf(stderr, "未知参数: %s\n", args[i])
			return 64
		}
	}
	// 四个「只出一件」档指向不同制品 ⇒ 同时给是用法错（不猜谁优先：猜错就编错东西）。
	nOnly := 0
	for _, b := range []bool{onlyCLI, onlyCore, onlyCompat, onlyAgentd} {
		if b {
			nOnly++
		}
	}
	if nOnly > 1 {
		fmt.Fprintln(stderr, "✗ --only-cli / --only-core / --only-compat / --only-agentd 互斥（各只写一件：bin/zerg · bin/zerg-core · bin/zerg-compat · bin/zerg-agentd）⇒ 用法错")
		return 64
	}
	// `--dist` 与 `--only-agentd` 靶子自相矛盾（--dist 的边界逐字是「只落 bin/ 的那三件一个都不产」）。
	if dist && onlyAgentd {
		fmt.Fprintln(stderr, "✗ --dist 与 --only-agentd 互斥（--dist 只写 dist/<版本>/、不产也不签 bin/ 的部署件）⇒ 用法错")
		return 64
	}
	buildUI := !noUI
	signOn := !noSign

	root := repoRootOverride
	if root == "" {
		root = zgb.DiscoverRepoRoot()
	}
	if root == "" {
		fmt.Fprintln(stderr, "❌ 推不出仓根（源码路径与工作目录上溯都找不到仓标记）⇒ 请给 --repo-root（本件不静默猜测）")
		return 2
	}
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}

	// ── 自净化（`build-all.sh:82-93`）：在解析参数与一切仓内读写之前 ──
	baseEnv, tmpdir, ok := zgb.SelfClean(root, stdout, stderr)
	if !ok {
		return 1
	}

	version := readVersion(root)
	if version == "" {
		fmt.Fprintln(stderr, "❌ 无法从 core/internal/version/version.go 读到版本号")
		return 1
	}
	sha := gitSha(root, baseEnv)
	buildTime := buildTimeNow()

	out := filepath.Join(root, "bin")
	if dist {
		out = filepath.Join(root, "dist", version)
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		fmt.Fprintf(stderr, "❌ 建输出目录失败 %s：%v\n", out, err)
		return 1
	}

	// 身份注入**第一份**：-ldflags（core 模块的 version 包）。
	ldflags := "-s -w -X github.com/Mr2109/zerg-swarm/core/internal/version.Commit=" + sha +
		" -X github.com/Mr2109/zerg-swarm/core/internal/version.BuildTime=" + buildTime
	// 身份注入**第二份**：进嵌入段（`-tags`）—— 不执行该件也能 `go version -m <件>` 读到。
	gotags := "commit=" + sha + ",build_time=" + buildTime
	// agent 模块自己的 version 包（Version/Commit/BuildTime 三格，与全量档那一行逐字同源）。
	agentLDFLAGS := "-s -w -X github.com/Mr2109/zerg-swarm/agent/internal/version.Version=" + version +
		" -X github.com/Mr2109/zerg-swarm/agent/internal/version.Commit=" + sha +
		" -X github.com/Mr2109/zerg-swarm/agent/internal/version.BuildTime=" + buildTime

	logf := func(format string, a ...any) { fmt.Fprintf(stdout, format+"\n", a...) }

	fmt.Fprintf(stdout, "🏷  版本 %s · 代码 %s · 构建 %s\n", version, sha, buildTime)
	fmt.Fprintf(stdout, "📁 输出 %s\n", out)

	hasCodesign := lookPath("codesign")
	hasCargo := lookPath("cargo")

	// goBuild 跑 `go build`：Dir=模块目录，env=自净化后 baseEnv ＋ GOFLAGS=-mod=mod / GOSUMDB=off（＋ extra）。
	goBuild := func(dir string, args []string, extra ...string) int {
		env := zgb.EnvAdd(baseEnv, "GOFLAGS=-mod=mod", "GOSUMDB=off")
		env = zgb.EnvAdd(env, extra...)
		cmd := exec.Command("go", append([]string{"build"}, args...)...)
		cmd.Dir = dir
		cmd.Env = env
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, stdout, stderr
		return runRC(cmd)
	}
	coreOut := func(name string) string { return filepath.Join(out, name) }
	coreBuild := func(dir, outBin, pkg string) int {
		return goBuild(filepath.Join(root, dir),
			[]string{"-trimpath", "-buildvcs=false", "-tags", gotags, "-ldflags", ldflags, "-o", outBin, pkg})
	}

	// ── --only-agentd：子端换件专用档（只写 bin/zerg-agentd 一个文件）─────────────────
	if onlyAgentd {
		fmt.Fprintln(stdout, "→ 子端 zerg-agentd（--only-agentd：只写 bin/zerg-agentd 一件）")
		if !archivePrevPiece(root, filepath.Join(root, "bin", "zerg-agentd"), stdout, stderr) {
			fmt.Fprintln(stderr, "!! 留档失败 ⇒ 拒换件（bin/zerg-agentd 尚未被覆盖，一个字节未动）")
			return 1
		}
		if rc := goBuild(filepath.Join(root, "agent"),
			[]string{"-trimpath", "-buildvcs=false", "-tags", gotags, "-ldflags", agentLDFLAGS, "-o", filepath.Join(root, "bin", "zerg-agentd"), "./cmd/zerg-agentd"}); rc != 0 {
			return rc
		}
		sign.One(sign.Resolve(signOn, tmpdir, logf), filepath.Join(root, "bin", "zerg-agentd"), logf)
		fmt.Fprintln(stdout, "✅ 构建完成（--only-agentd：只写 bin/zerg-agentd；bin/zerg-core / bin/zerg / build-info.json 一个字节未动）")
		fmt.Fprintf(stdout, "   %-14s %d 字节\n", "zerg-agentd", sizeOf(filepath.Join(root, "bin", "zerg-agentd")))
		return 0
	}

	// ── --only-core：换主控专用档（只写 bin/zerg-core 一个文件）──────────────────────
	if onlyCore {
		fmt.Fprintln(stdout, "→ 主控 zerg-core（--only-core）")
		if rc := coreBuild("core", coreOut("zerg-core"), "./cmd/zerg-core"); rc != 0 {
			return rc
		}
		sign.One(sign.Resolve(signOn, tmpdir, logf), coreOut("zerg-core"), logf)
		fmt.Fprintln(stdout, "✅ 构建完成（--only-core：只写 bin/zerg-core；其余制品与 build-info.json 一个字节未动）")
		fmt.Fprintf(stdout, "   %-14s %d 字节\n", "zerg-core", sizeOf(coreOut("zerg-core")))
		return 0
	}

	// ── --only-compat：只出兼容层一件（回滚前置校验用）──────────────────────────────
	if onlyCompat {
		fmt.Fprintln(stdout, "→ 兼容层 zerg-compat（--only-compat）")
		if rc := coreBuild("core", coreOut("zerg-compat"), "./cmd/zerg-compat"); rc != 0 {
			return rc
		}
		sign.One(sign.Resolve(signOn, tmpdir, logf), coreOut("zerg-compat"), logf)
		fmt.Fprintln(stdout, "✅ 构建完成（--only-compat：只写 bin/zerg-compat；其余制品与 build-info.json 一个字节未动）")
		fmt.Fprintf(stdout, "   %-14s %d 字节\n", "zerg-compat", sizeOf(coreOut("zerg-compat")))
		return 0
	}

	// ── 命令面 zerg（薄壳 · 独立客户端二进制 · §6.1）─────────────────────────────────
	// 留档只挂这一处（`--only-cli` 与全量档共用同一行 `go build … -o "$OUT/zerg"`）；
	// --dist 档不挂：它写 dist/<版本>/，不是换生产件。
	if !dist {
		if !archivePrevPiece(root, filepath.Join(out, "zerg"), stdout, stderr) {
			fmt.Fprintf(stderr, "!! 留档失败 ⇒ 拒换件（%s/zerg 尚未被覆盖，一个字节未动）\n", out)
			return 1
		}
	}
	fmt.Fprintln(stdout, "→ 命令面 zerg（薄壳 · core/cmd/zerg）")
	if rc := coreBuild("core", coreOut("zerg"), "./cmd/zerg"); rc != 0 {
		return rc
	}
	if onlyCLI {
		sign.One(sign.Resolve(signOn, tmpdir, logf), coreOut("zerg"), logf)
		fmt.Fprintln(stdout, "✅ 构建完成（--only-cli：只写 bin/zerg；其余制品与 build-info.json 一个字节未动）")
		fmt.Fprintf(stdout, "   %-14s %d 字节\n", "zerg", sizeOf(coreOut("zerg")))
		return 0
	}

	fmt.Fprintln(stdout, "→ 主控 zerg-core")
	if rc := coreBuild("core", coreOut("zerg-core"), "./cmd/zerg-core"); rc != 0 {
		return rc
	}
	fmt.Fprintln(stdout, "→ 子端 zerg-agent")
	if rc := coreBuild("core", coreOut("zerg-agent"), "./cmd/zerg-agent"); rc != 0 {
		return rc
	}
	fmt.Fprintln(stdout, "→ 兼容层 zerg-compat")
	if rc := coreBuild("core", coreOut("zerg-compat"), "./cmd/zerg-compat"); rc != 0 {
		return rc
	}

	// ── 只落 bin/ 的三件（zerg-agentd · cocoon-docs-service · zerg-wall）：--dist 一档整块不跑 ──
	if dist {
		fmt.Fprintf(stdout, "→ 跳过 zerg-agentd / cocoon-docs-service / zerg-wall（--dist：本档只写 %s，不写 bin/、不重签 bin/）\n", out)
	} else {
		fmt.Fprintln(stdout, "→ 守护进程 zerg-agentd（agent 模块，注入 agent 自己的 version 包）")
		if rc := goBuild(filepath.Join(root, "agent"),
			[]string{"-trimpath", "-buildvcs=false", "-tags", gotags, "-ldflags", agentLDFLAGS, "-o", filepath.Join(root, "bin", "zerg-agentd"), "./cmd/zerg-agentd"}); rc != 0 {
			return rc
		}
		fmt.Fprintln(stdout, "→ 跨架构件 zerg-agentd-linux-amd64（linux/amd64）")
		if rc := goBuild(filepath.Join(root, "agent"),
			[]string{"-trimpath", "-buildvcs=false", "-tags", gotags, "-ldflags", agentLDFLAGS, "-o", filepath.Join(root, "bin", "zerg-agentd-linux-amd64"), "./cmd/zerg-agentd"},
			"GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=0", "GOTOOLCHAIN=local"); rc != 0 {
			return rc
		}
		fmt.Fprintln(stdout, "→ 跨架构件 zerg-agentd-darwin-amd64（darwin/amd64 · 实测可编、Rosetta 2 可跑）")
		if rc := goBuild(filepath.Join(root, "agent"),
			[]string{"-trimpath", "-buildvcs=false", "-tags", gotags, "-ldflags", agentLDFLAGS, "-o", filepath.Join(root, "bin", "zerg-agentd-darwin-amd64"), "./cmd/zerg-agentd"},
			"GOOS=darwin", "GOARCH=amd64", "CGO_ENABLED=0", "GOTOOLCHAIN=local"); rc != 0 {
			return rc
		}

		// 文档茧自带 Go 服务（**独立仓** `zerg-cocoon/文档/service`）：只构建不启动、只落 bin/、
		// 绝不交叉编译（非 darwin-arm64 直接跳过）；<container-repo>不在 / 无 go ⇒ 打印原因后跳过，**不阻断**。
		cocoonDir := filepath.Join(root, "..", "zerg-cocoon", "文档", "service")
		if zgb.UnameOS() == "Darwin" && zgb.UnameArch() == "arm64" && zgb.DirExists(cocoonDir) && lookPath("go") {
			fmt.Fprintln(stdout, "→ 文档茧自带服务 cocoon-docs-service（独立仓）")
			if rc := goBuild(cocoonDir,
				[]string{"-trimpath", "-buildvcs=false", "-o", filepath.Join(root, "bin", "cocoon-docs-service"), "."}); rc != 0 {
				return rc
			}
		} else {
			fmt.Fprintln(stdout, "→ 跳过 cocoon-docs-service（非 darwin-arm64 本机 / <container-repo>不在 / 无 go）")
		}

		// 茧壁 zerg-wall（批 2'.6）：**随卵分发**的沙箱外壳。委派 **Go 真身** `cmd/zerg-build`
		// （`build-wall.sh` 的 Go 化对等体 · 现编现跑 · 与 build-all.sh:426 同一块茧壁接线）。
		if hasCargo {
			fmt.Fprintln(stdout, "→ 茧壁 zerg-wall（随卵分发；不进 dist 制品矩阵）")
			wall := exec.Command("go", "run", "./cmd/zerg-build")
			wall.Dir = filepath.Join(root, "scripts", "build")
			wall.Env = baseEnv
			wall.Stdin, wall.Stdout, wall.Stderr = os.Stdin, stdout, stderr
			if rc := runRC(wall); rc != 0 {
				return rc
			}
		} else {
			fmt.Fprintln(stdout, "→ 跳过茧壁 zerg-wall（本机无 cargo）")
		}
	}

	// ── UI zerg-ui（cargo release）────────────────────────────────────────────────
	if buildUI {
		fmt.Fprintln(stdout, "→ UI zerg-ui（cargo release）")
		uiArgs := []string{"build", "--release", "-p", "zerg-ui"}
		if public {
			uiArgs = append(uiArgs, "--no-default-features")
		}
		cmd := exec.Command("cargo", uiArgs...)
		cmd.Dir = filepath.Join(root, "ui")
		cmd.Env = baseEnv
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, stdout, stderr
		if rc := runRC(cmd); rc != 0 {
			return rc
		}
		if err := zgb.CopyPreserve(filepath.Join(root, "ui", "target", "release", "zerg-ui"), coreOut("zerg-ui")); err != nil {
			fmt.Fprintf(stderr, "❌ 拷贝 UI 失败：%v\n", err)
			return 1
		}
	}

	// ── 收尾批量重签（稳定身份 + 固定 identifier）──────────────────────────────────
	if signOn && hasCodesign {
		list := []string{coreOut("zerg"), coreOut("zerg-core"), coreOut("zerg-agent"), coreOut("zerg-compat"), coreOut("zerg-ui")}
		if !dist {
			list = append(list,
				filepath.Join(root, "bin", "zerg-agentd"),
				filepath.Join(root, "bin", "cocoon-docs-service"),
				filepath.Join(root, "bin", "zerg-wall"),
				filepath.Join(root, "bin", "zerg-agentd-linux-amd64"),
				filepath.Join(root, "bin", "zerg-agentd-darwin-amd64"))
		}
		r := sign.Resolve(signOn, tmpdir, logf)
		for _, b := range list {
			sign.One(r, b, logf)
		}
	}

	// ── 构建身份落盘（打包 / 升级器读它 —— 不靠二进制自报，因为交叉编译的 linux 件跑不起来）──
	if err := writeBuildInfo(coreOut("build-info.json"), version, sha, buildTime); err != nil {
		fmt.Fprintf(stderr, "❌ 写 build-info.json 失败：%v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "   📝 build-info.json: version=%s commit=%s\n", version, sha)

	if dist {
		fmt.Fprintln(stdout, "→ 生成校验和（sha256）")
		if rc := writeChecksums(out, stdout, stderr); rc != 0 {
			return rc
		}
	}

	fmt.Fprintln(stdout, "✅ 构建完成")
	matches, _ := filepath.Glob(filepath.Join(out, "zerg-*"))
	sort.Strings(matches)
	for _, m := range matches {
		fmt.Fprintf(stdout, "   %-14s %d 字节\n", filepath.Base(m), sizeOf(m))
	}
	if !dist {
		cs := filepath.Join(root, "bin", "cocoon-docs-service")
		if zgb.FileExists(cs) {
			fmt.Fprintf(stdout, "   %-14s %d 字节（文档茧自带服务）\n", "cocoon-docs-service", sizeOf(cs))
		}
	}
	return 0
}

// ── 换件留档（`build-all.sh:186-248` 逐字同源）──────────────────────────────────
//
// history_drop_ok：留档落点必须落 `bin/_history/` 之下（产出面根里平铺 *.bak-* ⇒ 判红）。
func historyDropOK(rel string, stderr io.Writer) bool {
	if strings.HasPrefix(rel, "bin/_history/") {
		return true
	}
	fmt.Fprintf(stderr, "   ✗ 留档落点 %s 不在 bin/_history/ 之下（产出面根里平铺 *.bak-* ⇒ 判红）\n", rel)
	return false
}

// archivePrevPiece 覆盖前留档旧件（**从不删任何留档**：只有 mkdir + cp，没有 rm/mv/truncate 口）。
// 失败 ⇒ 返回 false（调用方拒换件）。
func archivePrevPiece(root, b string, stdout, stderr io.Writer) bool {
	if !zgb.FileExists(b) {
		return true // 目标还没有件（首建）⇒ 无可留档
	}
	base := filepath.Base(b)
	hist := filepath.Join(root, "bin", "_history")
	if err := os.MkdirAll(hist, 0o755); err != nil {
		fmt.Fprintf(stderr, "   ✗ 留档目录建不起来：%s ⇒ 拒换件\n", hist)
		return false
	}
	_, src := zgb.SizeAndSHA(b)
	hits, _ := filepath.Glob(filepath.Join(hist, base+".bak-*"))
	for _, hit := range hits {
		if !zgb.FileExists(hit) {
			continue
		}
		if _, hsha := zgb.SizeAndSHA(hit); hsha == src {
			fmt.Fprintf(stdout, "   🗄  留档跳过：%s 与已有留档 %s 逐字节相同（sha256 %s）—— 不重复留、不覆盖\n", base, filepath.Base(hit), src)
			return true
		}
	}
	stamp := time.Now().Format("20060102-150405")
	newp := filepath.Join(hist, base+".bak-"+stamp)
	if _, err := os.Stat(newp); err == nil { // 同一秒内二次调用 ⇒ 追加 pid，绝不覆盖已有留档
		newp = newp + "." + strconv.Itoa(os.Getpid())
	}
	if !historyDropOK(zgb.RelTo(root, newp), stderr) {
		return false
	}
	if err := zgb.CopyPreserve(b, newp); err != nil {
		fmt.Fprintf(stderr, "   ✗ 留档失败（cp -p %s → %s）：%v ⇒ 拒换件\n", b, newp, err)
		return false
	}
	_, sha := zgb.SizeAndSHA(newp) // 复制后回读 sha256（不拿 cp 的退出码当唯一判据）
	if sha != src {
		fmt.Fprintf(stderr, "   ✗ 留档回读 sha 与旧件不同（%s ≠ %s）：%s ⇒ 拒换件\n", sha, src, filepath.Base(newp))
		return false
	}
	sz, _ := zgb.SizeAndSHA(newp)
	fmt.Fprintf(stdout, "   🗄  已留档旧件：bin/_history/%s（%d 字节 · sha256 %s）\n", filepath.Base(newp), sz, src)
	return true
}

// ── 读版本 / 代码 / 构建时间 ─────────────────────────────────────────────────
func readVersion(root string) string {
	b, err := os.ReadFile(filepath.Join(root, "core", "internal", "version", "version.go"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(line, "const Version = ") {
			continue
		}
		start := strings.Index(line, "\"")
		end := strings.LastIndex(line, "\"")
		if start < 0 || end <= start {
			return ""
		}
		return line[start+1 : end]
	}
	return ""
}

func gitSha(root string, env []string) string {
	rev := exec.Command("git", "rev-parse", "--short", "HEAD")
	rev.Dir, rev.Env = root, env
	out, err := rev.Output()
	sha := strings.TrimSpace(string(out))
	if err != nil || sha == "" {
		return "unknown"
	}
	st := exec.Command("git", "status", "--porcelain")
	st.Dir, st.Env = root, env
	if o, _ := st.Output(); len(strings.TrimSpace(string(o))) > 0 {
		sha += "+dirty"
	}
	return sha
}

// buildTimeNow：缺省 = 现时 UTC（与 `build-all.sh:140` 逐字同）；给了 SOURCE_DATE_EPOCH 则冻结
// （业界可复现口径 —— 未签名件可争取逐字节同；见设计稿 §四 订正）。
func buildTimeNow() string {
	if sde := os.Getenv("SOURCE_DATE_EPOCH"); sde != "" {
		if n, err := strconv.ParseInt(sde, 10, 64); err == nil {
			return time.Unix(n, 0).UTC().Format("2006-01-02T15:04:05Z")
		}
	}
	return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}

func writeBuildInfo(path, version, sha, buildTime string) error {
	b, err := json.MarshalIndent(buildInfo{
		Version: version, Commit: sha, BuildTime: buildTime,
		BuilderHost: zgb.UnameOS() + "/" + zgb.UnameArch(),
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func writeChecksums(out string, stdout, stderr io.Writer) int {
	matches, _ := filepath.Glob(filepath.Join(out, "zerg-*"))
	sort.Strings(matches)
	var sb strings.Builder
	for _, m := range matches {
		_, sha := zgb.SizeAndSHA(m)
		line := fmt.Sprintf("%s  %s\n", sha, filepath.Base(m))
		sb.WriteString(line)
		fmt.Fprintf(stdout, "   %s", line)
	}
	if err := os.WriteFile(filepath.Join(out, "checksums.txt"), []byte(sb.String()), 0o644); err != nil {
		fmt.Fprintf(stderr, "❌ 写 checksums.txt 失败：%v\n", err)
		return 1
	}
	return 0
}

// ── 小工具 ───────────────────────────────────────────────────────────────────
func runRC(cmd *exec.Cmd) int {
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode()
		}
		return 1
	}
	return 0
}

func sizeOf(p string) int64 {
	fi, err := os.Stat(p)
	if err != nil {
		return 0
	}
	return fi.Size()
}

func lookPath(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
