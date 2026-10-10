// root.go —— 仓根解析（供门禁直通与帮助导出用）。
//
// 红线（scripts/gates/check-hardcoded-private-paths.py）：代码里**不许写死私有绝对路径**
// （`<volume-path>…` / `/Users/<user>…`）—— 公开快照会把它们替换成占位符，机群上的二进制
// 就指向不存在的目录。故这里只按三条**可推导**的路走：环境变量 → 可执行文件所在目录上溯 →
// 当前目录上溯；判据是「仓根有 core/internal/version/version.go」。
//
// ★ 危险面（本单只登记 · 零行为变化 · 缺口 `GAP-20260927-436/437/438`）：第 16 行那枚 `ZERG_REPO`
// 是**无条件采信**的 —— 设了就**换掉仓根**，不校验它是不是仓根（也不回退到下面两条推导路）。
// 指到非仓根时下游表现**不一致**（2026-09-27 按件现读）：`gate run` / `gate ls` 硬错 → rc=2
// （门禁最小入口不在）；`repo status` / `repo commit` 硬错 → rc=8（所指位置下没有 .git）；
// `doctor` 对**非仓根**（无仓根判据件）已报 BLOCKED（缺口 `GAP-20260927-436`）；而**公开树物料化快照**
// 因带同一枚判据件曾被误报「仓库根 **PASS**」⇒ 由 `isRepoRoot` 的**快照识别**收口（缺口 `GAP-20261006-08`）。
// ⇒ **写面 / 门禁动作请显式 `env -u ZERG_REPO`**。脚本侧同族旋钮：`ZERG_REPO_ROOT`（`scripts/gates/`
// 下 6 处 `REPO_ROOT` 覆盖；与命令面这两枚**不是同一个名字、也无单一真源**）。
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// repoRoot 返回仓根；解析不到返回空串（调用方决定报 BLOCKED 而不是猜一个）。
func repoRoot() string {
	if v := os.Getenv("ZERG_REPO"); v != "" {
		return v
	}
	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			if r := findRoot(filepath.Dir(resolved)); r != "" {
				return r
			}
		}
	}
	if wd, err := os.Getwd(); err == nil {
		if r := findRoot(wd); r != "" {
			return r
		}
	}
	return ""
}

func findRoot(dir string) string {
	for {
		if isRepoRoot(dir) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// isRepoRoot 判 dir 是不是 **zerg 真仓根**（不是「找得到仓根判据件」就算了）。
//
// 判据**两条都要**成立：
//
//	① 仓根判据件在：`<dir>/core/internal/version/version.go`；
//	② **不是公开树物料化快照** —— 发布链把公开树摊平后 `git init` 出来的产物，它**也**带同一枚
//	   version.go、也**有** `.git`，只按 ① 判会把它当仓根（静默换源；缺口 `GAP-20261006-08`：
//	   `ZERG_REPO` 指快照 ⇒ `repo status` 收为仓根 · `doctor` 报「仓库根 PASS」）。
//
// ② 的认法（真源现读定案 · `scripts/build/publish-public.sh` §2/§6「套用发布专属文件」段）：
// 快照根上那几件**只由发布链种下、真仓根没有** —— 出现**任一** ⇒ 判快照：
//
//	· `install.sh`                ← 来自 `publish/install.sh`
//	· `.gitleaks.toml`            ← 来自 `publish/gitleaks.toml`
//	· `gateway/fleet.example.yaml` ← 由私有的 `gateway/fleet.yaml` **改名**而来
//
// ★ 只认**根**相对路径（真仓里的 `publish/install.sh` 一类不受影响）。
func isMaterializedPublicTree_r42(dir string) bool {
	for _, rel := range []string{
		"install.sh",
		".gitleaks.toml",
		filepath.Join("gateway", "fleet.example.yaml"),
	} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err == nil {
			return true
		}
	}
	return false
}

func isRepoRoot(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, "core", "internal", "version", "version.go")); err != nil {
		return false
	}
	return !isMaterializedPublicTree_r42(dir)
}

// ---- `zerg root`（缺口 `GAP-20260925-43` · 顶层动作：**只印仓根一行**）----

// rootVia* —— `via` 的**取值域**（闭集 · 与三条路一一对应）。口径：如实报「**这一次**是从哪条路
// 拿到的仓根」，不是「今天有哪些路可用」。
const (
	rootViaEnv = "env"    // 采信了非空且**校验通过**的 `ZERG_REPO`
	rootViaExe = "可执行件上溯" // 从 `os.Executable()` 所在目录上溯命中
	rootViaCwd = "cwd 上溯" // 从 cwd 上溯命中
)

// rootInputs —— 一次仓根解析的三路输入。抽出来是为了能**进程内喂夹具**做正/负控：
// `os.Getenv` / `os.Executable` / `os.Getwd` 在测试里都不可注入。
type rootInputs struct {
	envRepo string // `ZERG_REPO` 原值（空串 = 未设）
	exeDir  string // 可执行件所在目录（已 EvalSymlinks；空串 = 不可得）
	wd      string // cwd（空串 = 不可得）
}

// resolveRepoRootFrom —— 三路解析仓根，**如实报 `via`**。返回 `(root, via, why)`：
// `root == ""` 即取不到（调用方 fail-closed）；`why` 只在失败时非空。
//
// ★ 与同包 `repoRoot()` 的**关键差别**（本单口径 ② · ✗ 别把两者并成一份）：
//
//	· `repoRoot()` **无条件采信** `ZERG_REPO`（设了就换源、不校验、不回落）—— 那半是另立的
//	  缺口 `GAP-20260927-436/437/438`，本单**不动它**（它被几十处门禁/命令调用，改它波及全局）。
//	· 本函数**校验** `ZERG_REPO` 指到的是不是真仓根：畸形 ⇒ **不给结论**（✗ 不静默采信该值、
//	  ✗ 也不回落 cwd 去凑一个更「顺眼」的仓根 —— 那会把「环境配错了」伪装成「一切正常」）。
func resolveRepoRootFrom(in rootInputs) (root, via, why string) {
	if in.envRepo != "" {
		if isRepoRoot(in.envRepo) {
			return in.envRepo, rootViaEnv, ""
		}
		return "", "", fmt.Sprintf("`ZERG_REPO` 指向的 %s **不是仓根**（无 `core/internal/version/version.go`，或像公开树物料化快照）—— fail-closed 不给结论（不静默采信该值、也不回落 cwd）", in.envRepo)
	}
	if in.exeDir != "" {
		if r := findRoot(in.exeDir); r != "" {
			return r, rootViaExe, ""
		}
	}
	if in.wd != "" {
		if r := findRoot(in.wd); r != "" {
			return r, rootViaCwd, ""
		}
	}
	return "", "", "三条路都没找到仓根（`ZERG_REPO` 未设 · 可执行件目录上溯无果 · cwd 上溯无果）"
}

// resolveRepoRoot —— 生产入口：把 os 三面读成 `rootInputs` 再解析（与 `repoRoot()` 同源三路）。
func resolveRepoRoot() (root, via, why string) {
	in := rootInputs{envRepo: os.Getenv("ZERG_REPO")}
	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			in.exeDir = filepath.Dir(resolved)
		}
	}
	if wd, err := os.Getwd(); err == nil {
		in.wd = wd
	}
	return resolveRepoRootFrom(in)
}

// cmdRoot —— `zerg root`：人面**只印仓根一行**；机器面给 `{root, via}`（登记见 `main.go`）。
// 取不到 ⇒ fail-closed（退码 8 · 出声），✗ 不许静默回落 cwd 当仓根。
//
// ★ 失败档与**既有 CLI 仓根缺失档同源**：`kind=blocked` / `detail=repo_root_absent` / 退码 8
// —— 与 `family_agent_ops.go` / `family_build.go` 等 6 处 `setErr("blocked","repo_root_absent",…)`
// 逐字同档（缺口 `GAP-20260925-43` 口径注[3] 的修法 ⑵：报「变量的值不是仓根」，✗ 别只报「没有 .git」）。
func cmdRoot(inv *invocation, stdout, stderr io.Writer) int {
	root, via, why := resolveRepoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", why)
		fmt.Fprintf(stderr, "%s: 解析不到仓根 —— %s\n", progName, why)
		fmt.Fprintf(stderr, "See '%s root --help'。\n", progName)
		emitErrIfJSON(inv, stdout, find([]string{"root"}), exitBlocked)
		return exitBlocked
	}
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			emitErrIfJSON(inv, stdout, find([]string{"root"}), rc)
			return rc
		}
		return selectJSON(stdout, stderr, inv, inv.path, inv.fields, map[string]string{"root": root, "via": via})
	}
	fmt.Fprintln(stdout, root)
	return exitOK
}

// RootResolveForTest —— 自检入口（正/负控都调它，不另抄一份解析逻辑）。
func RootResolveForTest(envRepo, exeDir, wd string) (root, via, why string) {
	return resolveRepoRootFrom(rootInputs{envRepo: envRepo, exeDir: exeDir, wd: wd})
}
