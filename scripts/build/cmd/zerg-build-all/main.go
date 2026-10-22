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
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
	// Origin —— 源提交（同源判定可比键 · 与注入进制品的 `-X …Origin=` 同一个值）。
	// `omitempty`：判不了（不是 git 仓）时**不写假值也不写空串**，老键一字不动，旧消费者零影响。
	Origin      string `json:"origin,omitempty"`
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
	origin := gitOrigin(root, baseEnv)
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
	// Origin（源提交）与 Commit/BuildTime 同一行注入：它是主控「同源判定」唯一的基准键，
	// 不注入 ⇒ `MasterOriginSHA()` 恒空串 ⇒ 全表 `same_origin` 永远 `unknown`（GAP-20261021-08）。
	ldflags := withOriginFlag("-s -w -X github.com/Mr2109/zerg-swarm/core/internal/version.Commit="+sha+
		" -X github.com/Mr2109/zerg-swarm/core/internal/version.BuildTime="+buildTime,
		"github.com/Mr2109/zerg-swarm/core/internal/version", origin)
	// 身份注入**第二份**：进嵌入段（`-tags`）—— 不执行该件也能 `go version -m <件>` 读到。
	gotags := "commit=" + sha + ",build_time=" + buildTime
	if origin != "" {
		// 第三格：源提交（与 -X 那一份同值，两个独立读法取或）。★ 必须**接在尾部**：
		// 全量收口互检按 `-tags=commit=<sha>,` 这个子串比（见下方四件互检），换顺序即假红。
		gotags += ",origin=" + origin
	}
	// agent 模块自己的 version 包（Version/Commit/BuildTime/Origin 四格，与全量档那一行逐字同源）。
	agentLDFLAGS := withOriginFlag("-s -w -X github.com/Mr2109/zerg-swarm/agent/internal/version.Version="+version+
		" -X github.com/Mr2109/zerg-swarm/agent/internal/version.Commit="+sha+
		" -X github.com/Mr2109/zerg-swarm/agent/internal/version.BuildTime="+buildTime,
		"github.com/Mr2109/zerg-swarm/agent/internal/version", origin)

	logf := func(format string, a ...any) { fmt.Fprintf(stdout, format+"\n", a...) }

	// ── GAP-20260927-311：--only-* 换件前置比对（fail-closed）─────────────────────
	// 盘上 bin/build-info.json 的 commit ≠ 本次算出的 sha ⇒ 拒跑任何 --only-* 换件档：
	// 那些档不写 build-info.json（逐字「其余制品与 build-info.json 一个字节未动」）⇒ 缺省
	// 一趟全量档对齐身份之前，--only-* 只会在混身份的 bin/ 上再叠一件。
	if onlyCLI || onlyCore || onlyCompat || onlyAgentd {
		if b, err := os.ReadFile(filepath.Join(root, "bin", "build-info.json")); err == nil {
			var bi buildInfo
			if err := json.Unmarshal(b, &bi); err == nil && bi.Commit != sha {
				fmt.Fprintf(stderr, "!! build-info.json 的 commit=%s ≠ 本次代码 %s ⇒ 拒换件（--only-* 不写 build-info.json；先跑一趟缺省全量 build-all 对齐身份再换）\n", bi.Commit, sha)
				return 1
			}
		} // build-info.json 读不到/解不动 ⇒ 不拦（全新仓首趟本来就没有它）
	}

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
		if sha == "unknown" {
			fmt.Fprintln(stderr, "!! 仓根不是 git 仓 ⇒ 无法核对 build-info.json 身份 ⇒ 拒换件（--only-agentd；在 git 仓内跑，或改用缺省全量档）")
			return 1
		}
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
	// ★ 本件是**派生件 · 非身份真源**（设计 §5 `O-12` 取（乙）· 2026-09-24 待拍清单终版 条 4/5）：
	//	- 全量档的**聚合**生成物（`version` ＋ provenance：`commit` / `build_time` / `builder_host`），
	//	  供打包 / 升级器读「这批制品是谁产的」—— 它是**记录**，✗ 不是身份。
	//	- **只有全量档写它**：`--only-cli` / `--only-core` / `--only-compat` / `--only-agentd` 各档
	//	  在本行之前 `return`（逐字「其余制品与 build-info.json 一个字节未动」）⇒ 那四档不碰它。
	//	- ✗ **勿手改**（下次全量档会被覆写）；✗ **勿当身份真源** —— 身份真源 = **制品自身嵌入段**
	//	  （`-tags commit=…,build_time=…`；读法 `zerg version --build-info`）。
	if err := writeBuildInfo(coreOut("build-info.json"), version, sha, buildTime, origin); err != nil {
		fmt.Fprintf(stderr, "❌ 写 build-info.json 失败：%v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "   📝 build-info.json: version=%s commit=%s origin=%s\n", version, sha, originOrUnknownWord(origin))

	// ── GAP-20260927-311：全量收口四件互检（fail-closed）────────────────────────
	// 四件制品内嵌身份（`-tags` 嵌入段，`go version -m` 可读）逐件 == 本次 sha —— 不齐 ⇒ 非零退：
	// 说明某件是旧趟残留（coreBuild/签名中途失败续跑、手放件等），✗ 不许当一趟齐的制品交出去。
	for _, name := range []string{"zerg", "zerg-core", "zerg-agent", "zerg-compat"} {
		b := coreOut(name)
		cmd := exec.Command("go", "version", "-m", b)
		cmd.Dir, cmd.Env = root, baseEnv
		outb, err := cmd.Output()
		want := "-tags=commit=" + sha + ","
		if err != nil || !strings.Contains(string(outb), want) {
			fmt.Fprintf(stderr, "❌ 四件互检失败：%s 内嵌 Commit ≠ 本次 %s（或读不出嵌入段）⇒ 非零退\n", name, sha)
			return 1
		}
	}

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

// runLedgerRel：跑门台账 —— 「每跑一趟门就追加一行」的运行期落点，**不是制品输入**。
//
// 制品身份（`zerg version` 第三格）只认**源码面**：把它算成脏，会让每一趟门之后的
// 重编都产出一枚 `+dirty` 制品，而正式面（命令页身份行 `Q-211`）按规矩拒收脏树产物
// —— 即「跟踪一个跑门就变动的日志」= 制品结构性判不了绿（2026-10-09 实测：四页 drift
// 全「无」、源码面「自 X 起无改动」，唯一红格就是后缀 `+dirty`）。
const runLedgerRel = "scripts/gates/judge-run-ledger.jsonl"

func gitSha(root string, env []string) string {
	rev := exec.Command("git", "rev-parse", "--short", "HEAD")
	rev.Dir, rev.Env = root, env
	out, err := rev.Output()
	sha := strings.TrimSpace(string(out))
	if err != nil || sha == "" {
		return "unknown"
	}
	// ★ 排除 runLedgerRel：台账每跑一趟门就追加 ⇒ 若计入，制品永远带 `+dirty`（见上方注）。
	st := exec.Command("git", "status", "--porcelain", "--", ".", ":(exclude)"+runLedgerRel)
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

// originShortLen —— 源提交（`version.Origin`）的**规范长度**，也是「两侧逐字可比」的唯一前提。
//
// 病灶形状：trailer 里写的是 **40 位全 sha**（`publish/mirror-public.sh` 的 `GitOrigin-RevId`），
// 而 `rev-parse --short HEAD` 给 **7~8 位** —— 主控（私有树 ⇒ HEAD）与节点（镜像树 ⇒ trailer）
// 不归一化就必然比成 `mixed`（假红），而假红会把「同源」这件事又变成判不了。
// 12 位：git 常用的「够区分」长度，又短到能进日志/心跳体。
const originShortLen = 12

// gitOrigin —— 读**源提交**（同源判定可比键 · GAP-20261013-09；注入面 GAP-20261021-08）：
//
//	① HEAD 带 `GitOrigin-RevId` trailer（镜像/中继树逐笔都带）⇒ 取 trailer 值 = 私有图源提交；
//	② 不带（私有权威树）⇒ 取 HEAD 本身；
//	③ 不是 git 仓 / 读不出 ⇒ 空串 = **判不了**（✗ 回落成 sha：镜像 tip 冒充源提交就是假判决）。
//
// ★ 一律截成 originShortLen 位；✗ 不带 `+dirty` —— 脏是**内容**不同（Digest 那格答），源提交仍是
// 那一笔，带上它会把同源判成 `mixed`。★ 两侧（core / agent）都由本函数注入 ⇒ 同一形状。
func gitOrigin(root string, env []string) string {
	if v := gitOriginTrailer(root, env); v != "" {
		return originShort(v)
	}
	return originShort(gitHeadSHA(root, env))
}

// gitOriginTrailer 读 HEAD 的 `GitOrigin-RevId` trailer（读不到 / 无该 trailer ⇒ 空串）。
// 多行（同名 trailer 出现多次）⇒ 取**第一条非空**，与 `%(trailers:key=…)` 的取值序一致。
func gitOriginTrailer(root string, env []string) string {
	cmd := exec.Command("git", "log", "-1", "--format=%(trailers:key=GitOrigin-RevId,valueonly)", "HEAD")
	cmd.Dir, cmd.Env = root, env
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if v := strings.TrimSpace(line); v != "" {
			return v
		}
	}
	return ""
}

// gitHeadSHA 取 HEAD 的**定长短** sha（`--short=12`；不是默认的 7~8）—— 与 trailer 那侧同长。
func gitHeadSHA(root string, env []string) string {
	cmd := exec.Command("git", "rev-parse", "--short="+strconv.Itoa(originShortLen), "HEAD")
	cmd.Dir, cmd.Env = root, env
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// originShort 把源提交截成 originShortLen 位（短于该长度 ⇒ 原样；空 ⇒ 空 = 判不了）。
func originShort(v string) string {
	v = strings.TrimSpace(v)
	if len(v) <= originShortLen {
		return v
	}
	return v[:originShortLen]
}

// withOriginFlag 把 `-X <pkg>.Origin=<值>` 接到一串 ldflags 后面。
// 值为空 ⇒ **一个字节也不接**（未注入 = 空串 = 判不了；不写一处看着像「配了」的 `Origin=`）。
func withOriginFlag(flags, pkgPath, origin string) string {
	if origin == "" {
		return flags
	}
	return flags + " -X " + pkgPath + ".Origin=" + origin
}

// originOrUnknownWord 人面词：空 ⇒ `unknown`（机器面仍写空串/省略，见 buildInfo.Origin）。
func originOrUnknownWord(origin string) string {
	if origin == "" {
		return "unknown"
	}
	return origin
}

func writeBuildInfo(path, version, sha, buildTime, origin string) error {
	b, err := json.MarshalIndent(buildInfo{
		Version: version, Commit: sha, BuildTime: buildTime, Origin: origin,
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
