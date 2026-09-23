// family_gate_bench.go —— `zerg gate bench`（§四 G1/§七 G1 · 缺口-命令面-20260921 §十一 P0-8）。
//
// 为什么它排 P0：门禁耗时今天靠 `time`（**7 行**），而设计稿 §九 M8 `U-M8-4` 逐字
// 「**契约里不许写任何门禁时长**」⇒ **时长未标定**，预算类判据（`SD4` 成本上限、
// `SD6` 回滚重试上限）**全悬空**。命令化之后：「快速档必须回 rc=0」这条验收硬线有了基线可比、
// 门禁变慢能被机器发现（今天只能靠人觉得慢）。
//
// 口径（照 §十一 P0-8 的形态，不自造）：
//
//	形态 `zerg gate bench [--fast | --scope <s>…] [--repeat n] [--json <字段>]`
//	输出逐趟 `run/real_ms/user_ms/sys_ms/rc/log` + 中位/最差 + **门禁身份**（脚本 sha256）
//	退码 `0` 量到 / `1` 有 FAIL（任何一趟非 0）/ `2` 用法错
//
// 三条口径：① **直通不翻译**（每一趟就是真跑门禁，退码照面报，不改写）；
// ② **身份现算**（脚本 sha256 随场次报出 —— 换了一件脚本，两批数字就不可比）；
// ③ **每趟一份日志**（门禁自己的规矩：一步一文件 ⇒ 本命令一趟一文件，不互相覆盖）。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func cmdGateBench(inv *invocation, stdout, stderr io.Writer) int {
	// ★ 用法错**先判**：`--json` 不给字段 ⇒ 当场 2 + stdout 0 字节（§4.1 K2 · `K2` 归一后取自退码表）——
	// 不许先把门禁真跑一趟（那是分钟级开销）再拿「字段没给」把人打回去。
	if inv.jsonGiven && len(inv.fields) == 0 {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
	}
	root := repoRoot()
	if root == "" {
		inv.setErr("usage", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 —— 量门禁需要仓根（找 core/internal/version/version.go 失败）\n", progName)
		return exitUsage
	}
	script := filepath.Join(root, gateScriptRel)
	if _, err := os.Stat(script); err != nil {
		inv.setErr("usage", "gate_script_absent", "门禁最小入口不在")
		fmt.Fprintf(stderr, "%s: 门禁最小入口不在（%s）—— 它是自举件，就地缺席即硬错\n", progName, gateScriptRel)
		return exitUsage
	}
	// 档位：`--fast` 与 `--scope` **互斥**（两种口径同时给 ⇒ 哪一个是它量的那一档说不清）。
	scopes := inv.flagVals("--scope")
	if inv.fast && len(scopes) > 0 {
		inv.setErr("usage", "fast_and_scope", "--fast 与 --scope 互斥")
		fmt.Fprintf(stderr, "%s: `--fast` 与 `--scope` 互斥（量的是哪一档必须唯一 · 退码 2）\n", progName)
		return exitUsage
	}
	repeat := 1
	if v := strings.TrimSpace(inv.flagVal("--repeat")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			inv.setErr("usage", "bad_repeat", "--repeat 不是正整数")
			fmt.Fprintf(stderr, "%s: `--repeat` 要正整数（给了 %q · 退码 2）\n", progName, v)
			return exitUsage
		}
		repeat = n
	}
	runArgs := []string{}
	label := "全量（默认档）"
	if inv.fast {
		runArgs, label = []string{"--fast"}, "快速档（--fast）"
	} else if len(scopes) > 0 {
		for _, s := range scopes {
			runArgs = append(runArgs, "--scope", s)
		}
		label = "scope=" + strings.Join(scopes, ",")
	}
	idSHA, idBytes := fileSHA256Of(script)

	logDir, err := os.MkdirTemp("", "zerg-gate-bench-")
	if err != nil {
		inv.setErr("failed", "tmpdir_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 建不了日志目录（%v）\n", progName, err)
		return exitFail
	}
	fmt.Fprintf(stderr, "%s: 门禁身份 %s（%d 字节 · 现算）· 档位 %s · 重复 %d 趟\n",
		progName, idSHA[:16]+"…", idBytes, label, repeat)

	rows := []map[string]string{}
	reals, users, syss := []float64{}, []float64{}, []float64{}
	bad := 0
	for i := 1; i <= repeat; i++ {
		logPath := filepath.Join(logDir, fmt.Sprintf("%02d.log", i))
		f, ferr := os.Create(logPath)
		if ferr != nil {
			inv.setErr("failed", "log_create_failed", ferr.Error())
			fmt.Fprintf(stderr, "%s: 第 %d 趟日志建不了（%v）\n", progName, i, ferr)
			return exitFail
		}
		cmd := exec.Command("bash", append([]string{script}, runArgs...)...)
		cmd.Dir = root
		cmd.Stdout, cmd.Stderr = f, f
		start := time.Now()
		runErr := cmd.Run()
		real := time.Since(start).Seconds() * 1000
		_ = f.Close()
		user, sys := procTimes(cmd)
		rc := 0
		if runErr != nil {
			if ee, ok := runErr.(*exec.ExitError); ok {
				rc = ee.ExitCode()
			} else {
				rc = 1
			}
		}
		if rc != 0 {
			bad++
		}
		reals, users, syss = append(reals, real), append(users, user), append(syss, sys)
		rows = append(rows, map[string]string{
			"run":     strconv.Itoa(i),
			"real_ms": fmt.Sprintf("%.0f", real),
			"user_ms": fmt.Sprintf("%.0f", user),
			"sys_ms":  fmt.Sprintf("%.0f", sys),
			"rc":      strconv.Itoa(rc),
			"log":     logPath,
		})
		fmt.Fprintf(stderr, "%s: 第 %d/%d 趟 rc=%d real=%.0fms user=%.0fms sys=%.0fms → %s\n",
			progName, i, repeat, rc, real, user, sys, logPath)
	}
	med := median(reals)
	worst := 0.0
	for _, v := range reals {
		if v > worst {
			worst = v
		}
	}
	rows = append(rows, map[string]string{
		"run":     "中位/最差",
		"real_ms": fmt.Sprintf("%.0f", med),
		"user_ms": fmt.Sprintf("%.0f", median(users)),
		"sys_ms":  fmt.Sprintf("%.0f", median(syss)),
		"rc":      strconv.Itoa(bad),
		"log":     fmt.Sprintf("real 中位 %.0fms · 最差 %.0fms · 门禁身份 %s · 档位 %s · 非零趟数 %d", med, worst, idSHA[:16]+"…", label, bad),
	})
	if rc := listCmd(inv, stdout, stderr, []string{"run", "real_ms", "user_ms", "sys_ms", "rc", "log"}, rows); rc != exitOK {
		return rc
	}
	if bad > 0 {
		fmt.Fprintf(stderr, "%s: 有 %d 趟非零（退码 1）—— 时长无效（量的是「跑挂了的门禁」），先修红再看数\n", progName, bad)
		return exitFail
	}
	return exitOK
}

// procTimes 取一趟的 user/sys CPU 时间（真源 = `Rusage`，不是自己累加）。
func procTimes(cmd *exec.Cmd) (userMS, sysMS float64) {
	if cmd.ProcessState == nil {
		return 0, 0
	}
	ru, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage)
	if !ok {
		return 0, 0
	}
	return float64(ru.Utime.Sec)*1000 + float64(ru.Utime.Usec)/1000,
		float64(ru.Stime.Sec)*1000 + float64(ru.Stime.Usec)/1000
}

// median 中位数（偶数个取中间两个的均值 —— 口径写死，不留「看情况」）。
func median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	cp := append([]float64{}, xs...)
	sort.Float64s(cp)
	m := len(cp) / 2
	if len(cp)%2 == 1 {
		return cp[m]
	}
	return (cp[m-1] + cp[m]) / 2
}

// fileSHA256Of 现算一件的 sha256 与字节数（门禁身份 —— 换了一件脚本，两批数字就不可比）。
func fileSHA256Of(p string) (string, int64) {
	b, err := os.ReadFile(p)
	if err != nil {
		return "（读不到）", 0
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), int64(len(b))
}
