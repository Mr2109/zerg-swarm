// Package zgb — 批-6 自举链构建器的公共面（仓根发现 / 进程环境 / 文件 / 自净化）。
//
// 设计依据：`~/.zerg/state/设计-批6-自举链-谁编谁-20261007.md` §四（落地序 · 第五步）。
// 拍板：决-06（独立 module `zerg-build`）· 决-07（签名面并入 Go 包）· 决-08（自举环保留）。
//
// 本包零 import（只标准库）⇒ 鸡生蛋深度 = 1（构建器不 import 任何被它构建的模块）。
package zgb

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// DiscoverRepoRoot 推仓根：首选【本源码文件的编译期路径】上溯找仓标记；拿不到再退回【工作目录上溯】。
//
// ★ 血证（试点沿用 · 设计稿 §四 ①）：本仓**根下 `wall/` 与 `scripts/wall/` 两处同名**
// （后者是真目录，不是链接）⇒ 只认 `wall/` 会在 `scripts/` 这一层**提前收口**、把仓根认成
// `<仓>/scripts`（实测把 `--dist` 跑成「茧壁源码不在 …/scripts/wall/Cargo.toml」）。
// ⇒ 仓标记必须**同时**含 `wall/Cargo.toml` 与 `scripts/build/`（后者只在真仓根下存在）。
func DiscoverRepoRoot() string {
	if _, file, _, ok := runtime.Caller(0); ok && filepath.IsAbs(file) {
		for d := filepath.Dir(file); ; {
			if IsRepoRoot(d) {
				return d
			}
			parent := filepath.Dir(d)
			if parent == d {
				break
			}
			d = parent
		}
	}
	if wd, err := os.Getwd(); err == nil {
		for d := wd; ; {
			if IsRepoRoot(d) {
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

// IsRepoRoot 判仓根：`wall/Cargo.toml` 与 `scripts/build/` 必须同时在场（见 DiscoverRepoRoot 的血证）。
func IsRepoRoot(d string) bool {
	return FileExists(filepath.Join(d, "wall", "Cargo.toml")) && DirExists(filepath.Join(d, "scripts", "build"))
}

// RelTo 复刻 `${OUT#"${REPO_ROOT}"/}`（仓内件印相对路径，仓外件印绝对路径）。
func RelTo(root, p string) string {
	prefix := root + string(os.PathSeparator)
	if strings.HasPrefix(p, prefix) {
		return p[len(prefix):]
	}
	return p
}

// EnvWith 用 key=val 覆盖环境里的同名项（先剔除旧项再加 —— Go 的 exec 原样透传 envp，
// 直接追加会让旧值继续生效）。可复现开关 / GOFLAGS 覆盖靠它钉死。
func EnvWith(env []string, key, val string) []string {
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

// EnvDrop 剔除若干 key（自净化：`unset ZERG_STATE_DIR ZERG_REPO ZERG_PROPOSAL_DIR`）。
func EnvDrop(env []string, keys ...string) []string {
	out := make([]string, 0, len(env))
	for _, e := range env {
		drop := false
		for _, k := range keys {
			if strings.HasPrefix(e, k+"=") {
				drop = true
				break
			}
		}
		if !drop {
			out = append(out, e)
		}
	}
	return out
}

// EnvAdd 追加若干 "K=V"（逐个走 EnvWith ⇒ 覆盖而非重复）。
func EnvAdd(env []string, kv ...string) []string {
	for _, e := range kv {
		i := strings.Index(e, "=")
		if i < 0 {
			continue
		}
		env = EnvWith(env, e[:i], e[i+1:])
	}
	return env
}

// SelfClean 自净化（`build-all.sh:82-93` 的 Go 化对等体）：
// 只清 ZERG_STATE_DIR / ZERG_REPO / ZERG_PROPOSAL_DIR 三枚旋钮（其余环境变量一个都不动），
// 并把 TMPDIR 指到**本仓之外**（先 mkdir -p）。返回给子进程用的干净 env、仓外 TMPDIR、以及是否成功。
// 基准可经 ZERG_BUILD_TMP_BASE 换，但落点落在本仓内时**拒用**（换回仓外缺省）。
func SelfClean(scriptRoot string, stdout, stderr io.Writer) ([]string, string, bool) {
	home := os.Getenv("HOME")
	if home == "" {
		home = "/tmp"
	}
	def := filepath.Join(home, ".zerg-build-tmp")
	base := os.Getenv("ZERG_BUILD_TMP_BASE")
	if base == "" {
		base = def
	}
	// 给进来的基准落在本仓内 ⇒ 换回仓外缺省（`build-all.sh:85-87`）。
	if scriptRoot != "" && strings.HasPrefix(base+string(os.PathSeparator), scriptRoot+string(os.PathSeparator)) {
		base = def
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		fmt.Fprintf(stderr, "══ 自净化失败：仓外 TMPDIR 建不起来（%s）⇒ 拒绝在未净化的环境里重编\n", base)
		return nil, "", false
	}
	env := EnvDrop(os.Environ(), "ZERG_STATE_DIR", "ZERG_REPO", "ZERG_PROPOSAL_DIR")
	env = EnvWith(env, "TMPDIR", base)
	fmt.Fprintf(stdout, "══ 自净化：已 unset ZERG_STATE_DIR / ZERG_REPO / ZERG_PROPOSAL_DIR 三枚旋钮 · 本次 TMPDIR=%s（仓外；脚本仓根 %s）\n", base, scriptRoot)
	return env, base, true
}

func FileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

func DirExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func IsExecutable(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir() && fi.Mode().Perm()&0o111 != 0
}

// UnameOS 复刻 `uname -s`。
func UnameOS() string {
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

// UnameArch 复刻 `uname -m`（Go 的 GOARCH 命名与 uname 在本仓目标平台一致：arm64 / amd64）。
func UnameArch() string { return runtime.GOARCH }

func SizeAndSHA(p string) (int64, string) {
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

// CopyPreserve ≈ `cp -p`：复制内容并保留权限与时间戳（ad-hoc 签名在 Mach-O 体内，随字节走）。
func CopyPreserve(src, dst string) error {
	fi, err := os.Stat(src)
	if err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".tmp-zergbuild"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, fi.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Chmod(tmp, fi.Mode().Perm()); err != nil {
		os.Remove(tmp)
		return err
	}
	_ = os.Chtimes(tmp, time.Now(), fi.ModTime())
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
