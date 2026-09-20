// cli_exec_test.go —— 命令面契约测试的**第三层**：纯 Go `os/exec` + **自写合成夹具**（不引依赖）。
//
// 为什么还要有这一层（§九 M17「三层测试落点」）：进程内的两层测的是**同一份进程状态** ——
// 而命令面真实的用法里有三件进程内测不到的东西：① 真二进制的退出码（`os.Exit` 之后才是终值）；
// ② **透传型命令**（`zerg gate …`）把口令原样交给被包的脚本、退码原样转出（薄壳不翻译）；
// ③ 「仓根从哪儿解析」这条路径（`ZERG_REPO` → 可执行文件上溯 → cwd 上溯）。
//
// 合成夹具的形状（**不碰真仓**）：一个临时仓根，里面只放两件判据要的东西 ——
//
//	`<夹具>/core/internal/version/version.go`（仓根判据：`repoRoot()` 认的就是它）
//	`<夹具>/scripts/gates/precommit-gates.sh`（被透传的脚本：退码由它自己定）
//
// 然后 `ZERG_REPO=<夹具>` 起真二进制 ⇒ 测「脚本退多少、命令面就退多少」。
//
// 纪律：`bin/zerg` 不在就 **t.Skip**（本层不自己构建 —— 构建一律走 `scripts/build/build-all.sh`）。
package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// syntheticRepo 建一个最小合成仓根：只有仓根判据件与被透传的脚本。
func syntheticRepo(t *testing.T, scriptBody string) string {
	t.Helper()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "core", "internal", "version", "version.go"),
		"package version\n")
	mustWrite(t, filepath.Join(root, "scripts", "gates", "precommit-gates.sh"),
		"#!/usr/bin/env bash\nset -u\n"+scriptBody)
	if err := os.Chmod(filepath.Join(root, "scripts", "gates", "precommit-gates.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// zergBinary 找真制品；找不到 ⇒ 交给调用方 Skip。
func zergBinary(t *testing.T) string {
	t.Helper()
	p := filepath.Join("..", "..", "..", "bin", "zerg")
	if _, err := os.Stat(p); err != nil {
		t.Skipf("bin/zerg 不在（本层不自己构建；构建走 scripts/build/build-all.sh --only-cli）：%v", err)
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

// execCase 起真二进制跑一条命令（stdin 一律空 ⇒ 无 TTY、零提示词）。
func execCase(t *testing.T, bin, repo string, argv ...string) (int, string, string) {
	t.Helper()
	cmd := exec.Command(bin, argv...)
	cmd.Env = append(os.Environ(), "ZERG_REPO="+repo)
	cmd.Stdin = strings.NewReader("")
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb
	rc := 0
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			rc = ee.ExitCode()
		} else {
			t.Fatalf("起不了 bin/zerg：%v", err)
		}
	}
	return rc, out.String(), errb.String()
}

// TestCLIExecPassthroughCarriesScriptExitCode —— **透传不翻译**（§九 M11 `G1-a`）：
// 合成脚本退 1 ⇒ 命令面退 1；退 2 ⇒ 命令面退 2（不是「非 0 一律 1」）。
func TestCLIExecPassthroughCarriesScriptExitCode(t *testing.T) {
	bin := zergBinary(t)
	for _, want := range []int{1, 2} {
		root := syntheticRepo(t, "case \"${1:-}\" in --list) echo '── 步骤清单（scope 模式 名称）──'; echo 'docs 合成步'; echo '共 1 步'; exit 0 ;; esac\nexit "+string(rune('0'+want))+"\n")
		rc, _, errb := execCase(t, bin, root, "gate", "run", "--scope", "gates")
		if rc != want {
			t.Errorf("合成脚本退 %d ⇒ 命令面退 %d（薄壳只转发、不翻译：两值必须相等）· stderr=%s", want, rc, errb)
		}
	}
}

// TestCLIExecPassthroughListIsVerbatim —— `--list` 逐字透传：命令面的输出**就是**脚本的输出。
func TestCLIExecPassthroughListIsVerbatim(t *testing.T) {
	bin := zergBinary(t)
	body := "case \"${1:-}\" in --list) echo '── 步骤清单（scope 模式 名称）──'; echo 'docs 合成步'; echo '共 1 步'; exit 0 ;; esac\nexit 0\n"
	root := syntheticRepo(t, body)
	rc, out, errb := execCase(t, bin, root, "gate", "ls")
	if rc != 0 {
		t.Fatalf("gate ls 退码 = %d（要 0）· stderr=%s", rc, errb)
	}
	if !strings.Contains(out, "docs 合成步") {
		t.Errorf("命令面没有把合成脚本的输出原样转出：%q", out)
	}
	scriptOut, err := exec.Command(filepath.Join(root, "scripts", "gates", "precommit-gates.sh"), "--list").Output()
	if err != nil {
		t.Fatal(err)
	}
	if out != string(scriptOut) {
		t.Errorf("`zerg gate ls` 与脚本直跑**逐字节不同**：\n命令面=%q\n脚本  =%q", out, string(scriptOut))
	}
}

// TestCLIExecNoTTYNoPrompt —— 无 TTY 下**零提示词**（§4.1 K6）：stdout/stderr 里不许出现问句。
func TestCLIExecNoTTYNoPrompt(t *testing.T) {
	bin := zergBinary(t)
	root := syntheticRepo(t, "exit 0\n")
	for _, argv := range [][]string{{"version"}, {"help"}, {"gate", "ls"}} {
		rc, out, errb := execCase(t, bin, root, argv...)
		if rc != 0 {
			t.Errorf("%v 退码 = %d（要 0）· stderr=%s", argv, rc, errb)
		}
		for _, pat := range []string{"[y/N]", "(y/n)", "按回车", "请输入", "确认吗", "password", "Password"} {
			if strings.Contains(out+errb, pat) {
				t.Errorf("%v 的输出里出现提示词 %q（无 TTY 必须零提示词）", argv, pat)
			}
		}
	}
}
