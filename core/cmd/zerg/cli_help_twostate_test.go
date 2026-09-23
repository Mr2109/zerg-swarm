// cli_help_twostate_test.go —— `--help` **分两态**（缺口 `Q-137` / `Q-141` · 组A）的机检。
//
// 判据（设计稿 `设计-CLI机器读面-v1.0` §1.3 坑 1/2 · §3.2 两条迁法 · §4 第 18/20 行）：
//
//	① 命令**在册** + `--help` ⇒ 退 `0`，stdout 出**该条用法串**，且**不含**全局头行；
//	② 命令**不在册** + `--help` ⇒ 退 `2`（usage），stderr 首行逐字 `未知命令`；
//	③ **同族同码**：`zerg frobnicate --help` 与 `zerg frobnicate` 退码相同、stderr 首行逐字相同；
//	④ 对照：`zerg --help` / `zerg help` 仍是全局树（裸帮助面没被改坏）。
//
// 为什么四格必须成对钉：坑 1 的病根是「两个不同真值被并成同一个形状」——只钉①或只钉②都证明不了
// 两态**分得开**；只有把「在册」「不在册」「裸 `--help`」三条并排比，才判得出「探针能不能拿
// `--help` 当存在性判据」（修前 `frobnicate --help` ⇒ `rc=0` + 全局头行 = 假绿）。
//
// 落点：`package main_test` + `zerg.RunForTest` —— 跑的是**当前源码**的行为（不是盘上旧制品），
// 与 `cli_gap_test.go` / `cli_matrix_test.go` 同一条路；全程只读，不起子进程、不碰状态目录。
package main_test

import (
	"bytes"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// helpGlobalHeader —— 全局头行（`helpText()` 第 1 行）的逐字前缀。它出现在 stdout 里 ⇒ 这条问法
// 又把「命令存在」与「没给命令」混成一个形状了（坑 1/2 判词）。
const helpGlobalHeader = "zerg —— 虫族命令面（唯一入口）"

func helpRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := zerg.RunForTest(argv, &out, &errb)
	return rc, out.String(), errb.String()
}

// helpFirstLine 取首行（判「首行逐字」用）。
func helpFirstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// TestCLIHelpTwoState_KnownCommandGetsOwnUsage —— 正控：在册命令的 `--help` ⇒ 该条用法串。
func TestCLIHelpTwoState_KnownCommandGetsOwnUsage(t *testing.T) {
	for _, c := range []struct{ path, wantUsage string }{
		{"gate show", "zerg gate show <步名> [--json <字段>]"},
		{"model ls", "zerg model ls [--json <字段>]"},
	} {
		argv := append(strings.Split(c.path, " "), "--help")
		rc, out, errb := helpRun(t, argv...)
		if rc != 0 {
			t.Errorf("`zerg %s --help` ⇒ 在册命令应退 0，得到 %d · stderr=%q", c.path, rc, errb)
		}
		if got := helpFirstLine(out); got != c.wantUsage {
			t.Errorf("`zerg %s --help` 首行应为该条用法串 %q，得到 %q", c.path, c.wantUsage, got)
		}
		if strings.Contains(out, helpGlobalHeader) {
			t.Errorf("`zerg %s --help` 的 stdout **不该**含全局头行（那正是坑 1/2 的假绿形状）：%q", c.path, out)
		}
	}
}

// TestCLIHelpTwoState_UnknownCommandIsUsageError —— 反控：不在册命令的 `--help` ⇒ 退 2 + 首行「未知命令」。
func TestCLIHelpTwoState_UnknownCommandIsUsageError(t *testing.T) {
	for _, argv := range [][]string{
		{"frobnicate", "--help"},
		{"model", "rm", "--help"},
	} {
		rc, out, errb := helpRun(t, argv...)
		if rc != 2 {
			t.Errorf("`zerg %v` ⇒ 不在册应退 2（usage），得到 %d · stderr=%q", argv, rc, errb)
		}
		if out != "" {
			t.Errorf("`zerg %v` ⇒ 不在册不许往 stdout 打任何东西（尤其不许打全局树），得到 %q", argv, out)
		}
		if !strings.HasPrefix(helpFirstLine(errb), "zerg: 未知命令") {
			t.Errorf("`zerg %v` ⇒ stderr 首行应为「未知命令」，得到 %q", argv, helpFirstLine(errb))
		}
	}
}

// TestCLIHelpTwoState_SameFamilySameCode —— 同族同码：加不加 `--help` 不许改变一条不在册命令的真值。
func TestCLIHelpTwoState_SameFamilySameCode(t *testing.T) {
	rcA, outA, errA := helpRun(t, "frobnicate")
	rcB, outB, errB := helpRun(t, "frobnicate", "--help")
	if rcA != 2 || rcB != 2 {
		t.Errorf("`frobnicate`(rc=%d) 与 `frobnicate --help`(rc=%d) 应同为 2", rcA, rcB)
	}
	if helpFirstLine(errA) != helpFirstLine(errB) {
		t.Errorf("两条的 stderr 首行应逐字相同：%q vs %q", helpFirstLine(errA), helpFirstLine(errB))
	}
	if outA != "" || outB != "" {
		t.Errorf("两条都不许往 stdout 打东西：%q / %q", outA, outB)
	}
}

// TestCLIHelpTwoState_BareHelpStillGlobalTree —— 对照：裸 `--help` / `help` 仍出全局树。
func TestCLIHelpTwoState_BareHelpStillGlobalTree(t *testing.T) {
	for _, argv := range [][]string{{"--help"}, {"help"}} {
		rc, out, errb := helpRun(t, argv...)
		if rc != 0 {
			t.Errorf("`zerg %v` ⇒ 应退 0，得到 %d · stderr=%q", argv, rc, errb)
		}
		if !strings.Contains(out, helpGlobalHeader) {
			t.Errorf("`zerg %v` ⇒ 全局头行应在（裸帮助面没被改坏），得到首行 %q", argv, helpFirstLine(out))
		}
	}
}
