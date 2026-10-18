// Command zerg-build — 虫族自研构建器（批-6 自举链 · 试点件：茧壁 zerg-wall）。
//
// 本件是 `scripts/build/build-wall.sh`（现读 102 行）的 Go 化对等体（试点）。
// 设计依据：`~/.zerg/state/设计-批6-自举链-谁编谁-20261007.md` §四「先行试点」。
// 拍板：决-06（新建独立 module · ✗ 不进 core）· 决-07（签名面）· 决-08（自举环）。
//
// 落点 / 形态（决-06）
//
//	· 独立 module = 本目录 `scripts/build/go.mod`（进 core 会与「被编的 core」同模块 ⇒ 自依赖）。
//	· 现编现跑：`cd scripts/build && go run ./cmd/zerg-build`。
//	· ✗ 不 import 任何被它构建的模块（core / agent / shared）⇒ 鸡生蛋深度 = 1。
//
// 决-07（签名面）：茧壁**不涉签名** —— 落 `bin/` 的稳定身份签名由 `build-all.sh` 的签名环节
//
//	（`lib-sign.sh`）统一施加；cargo 产出的件自带 ad-hoc 签名、能跑。本件 ✗ 不复制第二份签名逻辑
//	（与 `build-wall.sh:25-26` 同口径）。
//
// 决-08（自举环）：本件**不依赖刚产出的新件** —— 只调外部 `cargo`，✗ 不 exec `bin/zerg`、
//
//	✗ 不 exec 刚编出的 `bin/zerg-wall`。环内的「引导器」永远是**已装的 cargo 工具链**。
//
// 用法：
//
//	go run ./cmd/zerg-build                     # release → <仓根>/bin/zerg-wall（本机平台）
//	go run ./cmd/zerg-build --debug             # debug   → <仓根>/wall/target/debug/zerg-wall
//	go run ./cmd/zerg-build --dist <目录>       # release → <目录>/zerg-wall（落仓外 · 供桩树/试点验收）
//	go run ./cmd/zerg-build --repo-root <仓根>   # 覆盖仓根（默认由本源码编译期路径上溯推出）
//
// 退出码：0 成功 · 2 硬失败（仓不完整 / 无 cargo / 编不出来 / 编完没有制品 / 制品起不来 / 用法错）。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"zerg-build/internal/zgb"
)

const usage = `用法: go run ./cmd/zerg-build [--debug] [--dist <目录>] [--repo-root <仓根>]
  release（默认）→ <仓根>/bin/zerg-wall（本机平台，不进 dist 制品矩阵）
  --debug        → <仓根>/wall/target/debug/zerg-wall（判据 7 的门禁脚本读它）
  --dist <目录>  → <目录>/zerg-wall（把 release 制品落仓外；✗ 不碰在跑的 bin/）
  --repo-root <仓根>  覆盖仓根（默认由本源码编译期路径上溯推出）
  --help | -h    → 印本用法`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run 是主体（输出走 writer ⇒ 可被负控/单测复放）。退出码 = 本件唯一出口。
func run(args []string, stdout, stderr io.Writer) int {
	profile := "release"
	distDir := ""
	repoRootOverride := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--debug":
			profile = "debug"
		case "--help", "-h":
			fmt.Fprintln(stdout, usage)
			return 0
		case "--dist":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "❌ --dist 缺目录参数（本件不静默忽略）")
				return 2
			}
			i++
			distDir = args[i]
		case "--repo-root":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "❌ --repo-root 缺路径参数（本件不静默忽略）")
				return 2
			}
			i++
			repoRootOverride = args[i]
		default:
			fmt.Fprintf(stderr, "❌ 未知参数: %s（本件只认 --debug / --dist / --repo-root / --help，不静默忽略）\n", args[i])
			return 2
		}
	}

	// --dist 只对 release 有意义（debug 落 <仓根>/wall/target/debug）；同给 ⇒ 用法错（不静默忽略）。
	if profile == "debug" && distDir != "" {
		fmt.Fprintln(stderr, "❌ --dist 与 --debug 互斥（debug 制品落 <仓根>/wall/target/debug，不进 dist）")
		return 2
	}

	root := repoRootOverride
	if root == "" {
		root = discoverRepoRoot()
	}
	if root == "" {
		fmt.Fprintln(stderr, "❌ 推不出仓根（源码路径与工作目录上溯都找不到仓标记）⇒ 请给 --repo-root（本件不静默猜测）")
		return 2
	}
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}

	wallDir := filepath.Join(root, "wall")

	// 仓不完整 = 硬失败：`wall/` 在发布白名单里 ⇒ 它不在就是仓坏了，不是「可以跳过」。
	if !fileExists(filepath.Join(wallDir, "Cargo.toml")) {
		fmt.Fprintf(stderr, "❌ 茧壁源码不在 %s/Cargo.toml ⇒ 仓不完整（wall/ 是仓内一等公民，不走跳过这条路）\n", wallDir)
		return 2
	}

	cargoPath, err := exec.LookPath("cargo")
	if err != nil {
		fmt.Fprintln(stderr, "❌ 本机没有 cargo（rust 工具链）⇒ 编不出茧壁；构建段见 wall/README.md")
		return 2
	}

	fmt.Fprintf(stdout, "→ 茧壁 zerg-wall（%s/%s，profile=%s）\n", unameOS(), unameArch(), profile)

	// CARGO_INCREMENTAL=0：dev profile 默认开增量编译 ⇒ 同一份源码连编两次 sha 不同（批 2'.5 实测）；
	// 关掉后逐字节相同 ⇒ 制品 sha 只有在可复现构建下才算证据，一律关（见 build-wall.sh:67-70）。
	buildArgs := []string{"build"}
	if profile == "release" {
		buildArgs = append(buildArgs, "--release")
	}
	buildCmd := exec.Command(cargoPath, buildArgs...)
	buildCmd.Dir = wallDir
	buildCmd.Env = envWith(os.Environ(), "CARGO_INCREMENTAL", "0")
	buildCmd.Stdin, buildCmd.Stdout, buildCmd.Stderr = os.Stdin, stdout, stderr
	if err := buildCmd.Run(); err != nil {
		fmt.Fprintf(stderr, "❌ cargo 失败（%v）⇒ 编不出茧壁\n", err)
		return 2
	}

	var artifact, out string
	if profile == "release" {
		artifact = filepath.Join(wallDir, "target", "release", "zerg-wall")
		if distDir != "" {
			out = filepath.Join(distDir, "zerg-wall")
		} else {
			out = filepath.Join(root, "bin", "zerg-wall")
		}
	} else {
		artifact = filepath.Join(wallDir, "target", "debug", "zerg-wall")
	}

	// 「编了」不等于「编出来了」：cargo 报成功之后必须看见制品。
	if !isExecutable(artifact) {
		fmt.Fprintf(stderr, "❌ cargo 说成功，但制品不在 %s —— 拒绝把「编了」当成「编出来了」\n", artifact)
		return 2
	}

	// 「有制品」还不等于「制品能起来」：`--version` 是茧壁自己的最小自检（不 exec 任何卵、不碰运行态）。
	verBytes, err := exec.Command(artifact, "--version").Output()
	if err != nil {
		fmt.Fprintf(stderr, "❌ 制品无法执行（rc!=0）：%s\n", artifact)
		return 2
	}
	version := strings.TrimRight(string(verBytes), "\n")

	if out != "" {
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			fmt.Fprintf(stderr, "❌ 建目录失败 %s：%v\n", filepath.Dir(out), err)
			return 2
		}
		if err := copyPreserve(artifact, out); err != nil {
			fmt.Fprintf(stderr, "❌ 拷贝失败 %s → %s：%v\n", artifact, out, err)
			return 2
		}
		if !isExecutable(out) {
			fmt.Fprintf(stderr, "❌ 拷贝后 %s 不可执行\n", out)
			return 2
		}
		size, sha := sizeAndSHA(out)
		fmt.Fprintf(stdout, "   ✅ %s  %s  %d 字节  sha256 %s\n", relTo(root, out), version, size, sha)
		fmt.Fprintln(stdout, "   ⓘ 签名：稳定身份签名由 build-all.sh 的签名环节统一施加（本件只编，不复制第二份签名逻辑）")
	} else {
		size, sha := sizeAndSHA(artifact)
		fmt.Fprintf(stdout, "   ✅ %s  %s  %d 字节  sha256 %s\n", relTo(root, artifact), version, size, sha)
	}
	return 0
}

// discoverRepoRoot 复刻 build-wall.sh 的 `$(dirname ${BASH_SOURCE})/../..` 口径：
// 首选【本源码文件的编译期路径】上溯到仓根（与脚本位置同义）；拿不到再退回【工作目录上溯】找仓标记。
// `go run` 未加 -trimpath ⇒ runtime.Caller 给的是绝对路径。
//
// ★ 血证：本仓**根下 `wall/` 与 `scripts/wall/` 两处同名**（后者是真目录，不是链接）⇒ 上溯口径
//
//	若只认 `wall/` 会在 `scripts/` 这一层就**提前收口**、把仓根认成 `<仓>/scripts`（实测把
//	`--dist` 跑成「茧壁源码不在 …/scripts/wall/Cargo.toml」）。故仓标记**必须同时**含
//	`wall/Cargo.toml` 与 `scripts/build/`（后者只在真仓根下存在）。
func discoverRepoRoot() string {
	// <root>/scripts/build/cmd/zerg-build/main.go ⇒ 上溯 4 层到 <root>。
	if _, file, _, ok := runtime.Caller(0); ok && filepath.IsAbs(file) {
		root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
		if dirExists(filepath.Join(root, "scripts", "build")) {
			return root
		}
	}
	if wd, err := os.Getwd(); err == nil {
		for d := wd; ; {
			if isRepoRoot(d) {
				return d
			}
			parent := filepath.Dir(d)
			if parent == d {
				break
			}
			d = parent
		}
	}
	return ""
}

// isRepoRoot 判仓根：`wall/Cargo.toml` 与 `scripts/build/` 必须同时在场（见 discoverRepoRoot 的血证）。
func isRepoRoot(d string) bool {
	return fileExists(filepath.Join(d, "wall", "Cargo.toml")) && dirExists(filepath.Join(d, "scripts", "build"))
}

// relTo 复刻 `${OUT#"${REPO_ROOT}"/}`（仓内件印相对路径，仓外件印绝对路径）。
func relTo(root, p string) string {
	prefix := root + string(os.PathSeparator)
	if strings.HasPrefix(p, prefix) {
		return p[len(prefix):]
	}
	return p
}

// envWith 用 key=val 覆盖环境里的同名项（先剔除旧项再加 —— Go 的 exec 原样透传 envp，
// 直接追加会让旧值继续生效）。CARGO_INCREMENTAL 的可复现开关靠它钉死。
func envWith(env []string, key, val string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			continue
		}
		out = append(out, e)
	}
	return append(out, prefix+val)
}

func isExecutable(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir() && fi.Mode().Perm()&0o111 != 0
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

func dirExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// copyPreserve ≈ `cp -p`：**委派** zgb.CopyPreserve（唯一的拷贝实现）。
//
// ★ 血证（2026-10-18 · GAP-20261018-26）：本件原先自带一份同形实现、临时件名是
// `dst + ".tmp-zergbuild"` ⇒ 与 build-all 的写者共用同一个临时名，并发时逐字报
// `rename <…>/zerg-wall.tmp-zergbuild <…>/zerg-wall: no such file or directory`
// 并退 2，`make build` 整条中止。两份实现必漂，故只留一份。
func copyPreserve(src, dst string) error { return zgb.CopyPreserve(src, dst) }

func sizeAndSHA(p string) (int64, string) {
	fi, err := os.Stat(p)
	if err != nil {
		return 0, ""
	}
	f, err := os.Open(p)
	if err != nil {
		return fi.Size(), ""
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fi.Size(), ""
	}
	return fi.Size(), hex.EncodeToString(h.Sum(nil))
}

// unameOS 复刻 `uname -s`（build-wall.sh 印「Darwin/arm64」形态）。
func unameOS() string {
	switch runtime.GOOS {
	case "darwin":
		return "Darwin"
	case "linux":
		return "Linux"
	case "windows":
		return "Windows"
	default:
		return runtime.GOOS
	}
}

func unameArch() string { return runtime.GOARCH }
