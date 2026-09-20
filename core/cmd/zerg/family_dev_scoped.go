// family_dev_test.go —— `zerg dev test --pkg <包> [--run <正则>]`（§三 C1 · 缺口-命令面-20260921 §十一 P0-3）。
//
// 为什么它排 P0：今天 **12 行手搓 `go test -run …`**，而 `zerg gate run --scope go` 一次跑整套
// （`gate ls` 现跑 go 步 19 条）—— **改一件要等整套**，自开发的最小回路（TDD 红绿）就转不起来。
// 命令化之后：改 → 只跑相关测 → 跑门，这条回路才闭合；`dev verify` 的结果表有更细粒度。
//
// 口径（照 §十一 P0-3 的形态，不自造 —— **不开新命令**，给已有的 `dev test` 加两枚旗标）：
//
//	形态 `zerg dev test --pkg <包> [--run <正则>] [--json <字段>]`
//	输出逐条 `step/verdict/rc/log_path` + 合计
//	退码 `0` 全绿 / `1` 有 FAIL / `2` 用法错（缺 --pkg / 包不在闭集 / 正则坏）/ `8` 跑不起来
//
// 三条口径：① **包名不猜**（认不出模块前缀 ⇒ 2 并列出四个合法前缀 —— 猜一个模块就是「换一件事跑」）；
// ② **正则先编译**（`go test -run` 用的是 Go 自己的正则语法，先判坏正则 ⇒ 2，别让 go 去报一句听不懂的）；
// ③ **一跑一份日志**（与门禁「一步一文件」同一规矩：日志路径进结果，谁都能回读）。
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// devTestModules —— `--pkg` 的模块前缀闭集（**不许猜**：认不出就退 2，不替人选一个模块）。
var devTestModules = []struct{ Prefix, Dir string }{
	{"core", "core"},
	{"agent", "agent"},
	{"shared", "shared"},
	{"scripts/exportnames", "scripts/exportnames"},
}

func cmdDevTestScoped(inv *invocation, stdout, stderr io.Writer) int {
	pkg := strings.TrimSpace(inv.flagVal("--pkg"))
	runRE := strings.TrimSpace(inv.flagVal("--run"))
	if pkg == "" {
		inv.setErr("usage", "missing_pkg", "缺 --pkg")
		fmt.Fprintf(stderr, "%s: `dev test --pkg <包>` 要给包（例：--pkg core/cmd/zerg）\n", progName)
		fmt.Fprintf(stderr, "四种合法前缀：core · agent · shared · scripts/exportnames\n")
		return exitUsage
	}
	// ② 正则先编译（Go 的 regexp 语法 = `go test -run` 的语法）。
	if runRE != "" {
		if _, err := regexp.Compile(runRE); err != nil {
			inv.setErr("usage", "bad_run_regex", err.Error())
			fmt.Fprintf(stderr, "%s: `--run` 正则编译不过：%v\n", progName, err)
			return exitUsage
		}
	}
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 跑不了测（不给结论 · 退码 8）\n", progName)
		return exitBlocked
	}
	// ① 包名不猜：前缀必须在闭集里。
	modDir, testsArg, ok := "", "", false
	for _, m := range devTestModules {
		if pkg == m.Prefix {
			modDir, testsArg, ok = m.Dir, "./...", true
			break
		}
		if strings.HasPrefix(pkg, m.Prefix+"/") {
			modDir, testsArg, ok = m.Dir, "./"+strings.TrimPrefix(pkg, m.Prefix+"/"), true
			break
		}
	}
	if !ok {
		inv.setErr("usage", "pkg_prefix_unknown", "包前缀不在闭集")
		fmt.Fprintf(stderr, "%s: 包 %q 的模块前缀认不出（**不替人猜**）⇒ 退码 2\n", progName, pkg)
		fmt.Fprintf(stderr, "四种合法前缀：core · agent · shared · scripts/exportnames（例：--pkg core/cmd/zerg）\n")
		return exitUsage
	}
	abs := filepath.Join(root, filepath.FromSlash(modDir))
	if st, err := os.Stat(abs); err != nil || !st.IsDir() {
		inv.setErr("blocked", "module_dir_absent", "模块目录不在")
		fmt.Fprintf(stderr, "%s: 模块目录不在（%s）⇒ 不给结论（退码 8）\n", progName, modDir)
		return exitBlocked
	}
	if _, err := exec.LookPath("go"); err != nil {
		inv.setErr("blocked", "go_absent", "本机没有 go")
		fmt.Fprintf(stderr, "%s: 本机没有 `go` ⇒ 跑不了测（不给结论 · 退码 8）\n", progName)
		return exitBlocked
	}
	args := []string{"test", testsArg}
	if runRE != "" {
		args = append(args, "-run", runRE)
	}
	args = append(args, "-count=1")

	logDir, err := os.MkdirTemp("", "zerg-dev-test-")
	if err != nil {
		inv.setErr("failed", "tmpdir_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 建不了日志目录（%v）\n", progName, err)
		return exitFail
	}
	logPath := filepath.Join(logDir, "01-go-test.log")
	f, err := os.Create(logPath)
	if err != nil {
		inv.setErr("failed", "log_create_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 日志建不了（%v）\n", progName, err)
		return exitFail
	}
	cmd := exec.Command("go", args...)
	cmd.Dir = abs
	cmd.Stdout, cmd.Stderr = f, f
	start := time.Now()
	runErr := cmd.Run()
	_ = f.Close()
	rc := 0
	if runErr != nil {
		if ee, ok := runErr.(*exec.ExitError); ok {
			rc = ee.ExitCode()
		} else {
			rc = 1
		}
	}
	verdict := "PASS"
	if rc != 0 {
		verdict = "FAIL"
	}
	step := fmt.Sprintf("%s: go %s", modDir, strings.Join(args, " "))
	fmt.Fprintf(stderr, "%s: %s ⇒ %s（rc=%d · %.1fs · 日志 %s）\n",
		progName, step, verdict, rc, time.Since(start).Seconds(), logPath)
	if rc != 0 {
		// 红的时候把**日志尾巴**原样贴出（判据要的是原文，不是「失败了」三个字）。
		if b, err := os.ReadFile(logPath); err == nil {
			tail := string(b)
			if len(tail) > 4000 {
				tail = tail[len(tail)-4000:]
			}
			fmt.Fprintln(stderr, "── 日志尾（原样）──")
			fmt.Fprint(stderr, tail)
		}
	}
	rows := []map[string]string{
		{"step": step, "verdict": verdict, "rc": fmt.Sprintf("%d", rc), "log_path": logPath},
		{"step": "合计", "verdict": verdict, "rc": fmt.Sprintf("%d", rc),
			"log_path": fmt.Sprintf("1 步 · %s · 日志目录 %s", verdict, logDir)},
	}
	if r := listCmd(inv, stdout, stderr, []string{"step", "verdict", "rc", "log_path"}, rows); r != exitOK {
		return r
	}
	if rc != 0 {
		fmt.Fprintf(stderr, "%s: 有 FAIL（退 1）—— 先看上面那一段日志原文；只跑相关的这一步是 **TDD 红绿环**的最小回路\n", progName)
		return exitFail
	}
	return exitOK
}
