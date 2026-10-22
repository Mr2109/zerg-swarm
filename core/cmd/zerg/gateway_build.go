// gateway_build.go —— `gateway/install.sh`（本机三件套构建）的 **Go 真身**（缺口 `GAP-20261015-01`）。
//
// 病（现读）：`gateway/install.sh` 31 行、零消费者（全仓无一处 exec 它；只在
// `publish/excludes.txt` / `scripts/build/publish-public.sh` 白名单 / `lang-freeze-baseline.json`
// 三处**登记面**出现）—— 而它在 `check-cli-ownership` 的读数是「例外放行 · 未 Go 化」
// （真源 `core/internal/contract/adopt-retire.json` · reason 逐字「未 Go 化：本机三件套构建」）。
// 「未 Go 化」是**时序**档（还没做），不是物理障碍 ⇒ 本件把它真搬进 Go。
//
// 判据（两态逐字）：原脚本的三条腿＝①目录缺席时逐字报「!! 目录不存在: <原样变量值>」；
// ②在场则在该目录里真跑 `swift build` / `./build.sh`（子进程继承 stdout/stderr · 退码原样转出）；
// ③收尾两行。本件逐条落 Go，**输出一个字节不加**（✗ 不加新行、✗ 不改文案、✗ 不换全角）。
//
// 未覆盖面（照实声明 · 实测两档）：子进程**起不来**（build.sh 缺件 / 无执行位）时 `stderr` 文案不同 ——
// bash 报 `install.sh: line 24: ./build.sh: No such file or directory`，本件报
// `gateway build: 起不来 ./build.sh：…`；**rc 与 stdout 仍逐字同**（实测两档 rc 皆 1 · stdout sha 同）。
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// gwbDirRel —— 原件所在目录（仓内相对）。原脚本首句 `cd "$(dirname "$0")"` ⇒ 相对路径以它为基准。
const gwbDirRel = "gateway"

// gwbResolve —— 把原脚本里 `<var>` 的取值解析成**可判/可跑**的路径（原脚本的 cwd = gateway/）。
//
// ★ 打印面一律用**原样变量值**（原脚本 `echo` 的是 `$MODEL_MANAGER_DIR` 未经归一化的值）⇒
// 本函数只喂 stat/exec，✗ 不参与任何一行输出。
func gwbResolve(base, v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if filepath.IsAbs(v) {
		return filepath.Clean(v)
	}
	return filepath.Join(base, v)
}

// gwbIsDir —— `[[ -d "$dir" ]]` 的等价判（空串 ⇒ false，与原脚本一致，不报 unbound）。
func gwbIsDir(dir string) bool {
	if dir == "" {
		return false
	}
	st, err := os.Stat(dir)
	return err == nil && st.IsDir()
}

// gwbRun —— 在 dir 里真跑一条命令，stdout/stderr 直通（等价 `(cd "$dir" && cmd…)`）。
// 退码：子进程退码原样转出（等价 `set -e` 打断）；起不来 ⇒ 1（见件头「未覆盖面」）。
func gwbRun(dir string, stdout, stderr io.Writer, name string, args ...string) int {
	c := exec.Command(name, args...)
	c.Dir = dir
	c.Stdout = stdout
	c.Stderr = stderr
	if err := c.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode()
		}
		fmt.Fprintf(stderr, "%s: 起不来 %s：%v\n", gwbProg, name, err)
		return 1
	}
	return 0
}

// gwbProg —— 报话前缀。
const gwbProg = "gateway build"

// cmdGatewayBuild —— `zerg gateway build` 的实现体（真搬 `gateway/install.sh`）。
//
// ★ 原脚本**不解析任何参数**（`$@` 一次都没读）⇒ 本件同样不解析：给什么参数都照跑，
// 两态在「带参数」这一格上也逐字同（✗ 不新造旗标面）。
func cmdGatewayBuild(inv *invocation, stdout, stderr io.Writer) int {
	base := filepath.Join(repoRoot(), gwbDirRel)

	// 原件：MODEL_MANAGER_DIR / X3_MONITOR_DIR（可选 · 缺席即空串）
	mmRaw := os.Getenv("MODEL_MANAGER_DIR")
	x3Raw := os.Getenv("X3_MONITOR_DIR")
	mm := gwbResolve(base, mmRaw)
	edge-node := gwbResolve(base, x3Raw)

	fmt.Fprintln(stdout, "==> [1/2] 构建模型管理器 App")
	if gwbIsDir(mm) {
		if rc := gwbRun(mm, stdout, stderr, "swift", "build"); rc != 0 {
			return rc
		}
		fmt.Fprintf(stdout, "    构建完成（swift build）。可运行: open %s/模型管理器.app\n", mmRaw)
	} else {
		fmt.Fprintf(stdout, "    !! 目录不存在: %s\n", mmRaw)
	}

	fmt.Fprintln(stdout, "==> [2/2] 构建 X3Monitor")
	if gwbIsDir(edge-node) {
		if rc := gwbRun(edge-node, stdout, stderr, "./build.sh"); rc != 0 {
			return rc
		}
		fmt.Fprintf(stdout, "    构建完成: %s/X3Monitor.app\n", x3Raw)
	} else {
		fmt.Fprintf(stdout, "    !! 目录不存在: %s\n", x3Raw)
	}

	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "==> 全部完成。子端部署请使用: ./deploy_x3.sh / ./deploy_mini.sh")
	return 0
}

// gatewayBuildGateMain —— `cmdGatewayBuild` 的 **inproc 同签名包装入口**（与 `gateCheckGoEntry`
// 表值同签名）。★ 只做 argv 归一后转发唯一实现 ⇒ 三面（stdout/stderr/退码）与命令面逐字同。
func gatewayBuildGateMain(stdout, stderr io.Writer, argv []string) int {
	inv := &invocation{
		path: []string{"gateway", "build"},
		orig: append([]string{"gateway", "build"}, argv...),
	}
	return cmdGatewayBuild(inv, stdout, stderr)
}
