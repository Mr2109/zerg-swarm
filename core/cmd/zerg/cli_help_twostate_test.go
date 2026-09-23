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

// ---- 组A 续（本批 · 2026-09-24）：`Q-154` 多余位置参数 · `Q-156` 用法串旗标 · `Q-149` 族级出口 ----
//
// 三格与上面四格**同一条纪律**：命令面不许把两种不同的真值并成同一个形状 ——
// `Q-154`「多给了」不许读成「给对了」· `Q-156`「用法串写的旗标」必须＝「真能用的旗标」·
// `Q-149` 族这一层此前**根本没有出口**（`model --help` 退 2 · `help model` 也退 2）。
// 每格都配**成对负控**（修前形态必须红）：只钉「改后对」证明不了「改前错」。

// noPosCommands —— 「声明不收位置参数」的**纯本机**代表集（`Q-154` 判据口那一格的样本）。
// 挑本机件是为了**公开面档**（仓根没有 `bin/`、没有主控）也照样跑得动 —— 样本只取可离线复现的。
var noPosCommands = []string{"version", "context ls", "calib ls", "plugin ls", "propose ls", "approve ls"}

// TestCLIQ154_ExtraPositionalIsUsageError —— 正控：多余位置参数 ⇒ 退 2 + stderr 首行逐字报出来。
// 病灶（修前现读实据）：`zerg version extraarg` 退 **0** —— 多余的那一枚被**静默吞**。
func TestCLIQ154_ExtraPositionalIsUsageError(t *testing.T) {
	for _, cmd := range noPosCommands {
		argv := append(strings.Split(cmd, " "), "zz-多余的")
		rc, out, errb := helpRun(t, argv...)
		if rc != 2 {
			t.Errorf("`zerg %s zz-多余的` ⇒ 要用法错 2，得到 %d · stderr=%q", cmd, rc, errb)
		}
		if first := helpFirstLine(errb); !strings.Contains(first, "多余位置参数") {
			t.Errorf("`zerg %s zz-多余的` 的 stderr 首行要逐字报「多余位置参数」，得到 %q", cmd, first)
		}
		if out != "" {
			t.Errorf("`zerg %s zz-多余的` 的 stdout 要 0 字节（执行前判 · 零副作用），得到 %q", cmd, out)
		}
	}
}

// TestCLIQ154_NegativeControl_NoExtraPositionalStillWorks —— 成对负控：**不给**多余位置参数时，
// 同一个判据口不许乱开火（否则新判据会把好输入一起拒掉 ⇒ 比原病更坏）。
func TestCLIQ154_NegativeControl_NoExtraPositionalStillWorks(t *testing.T) {
	for _, cmd := range noPosCommands {
		rc, _, errb := helpRun(t, strings.Split(cmd, " ")...)
		if rc == 2 && strings.Contains(errb, "多余位置参数") {
			t.Errorf("`zerg %s`（**没给**多余位置参数）不该被判成那一条：rc=%d · stderr=%q", cmd, rc, errb)
		}
	}
}

// TestCLIQ154_WaiverFace_DangerAndPassthroughUntouched —— 边界面：**危险档与透传型不在**这条判据里。
// 危险档的目标语义在 `guard.go` 一处（位置参数可当目标），透传型的位置参数逐字交脚本（§6.3 S3）
// ⇒ 它们**不许**报「多余位置参数」（本批一条都不动 · 逐条在案）。
// 这一格是**成对负控的正身**：证明改的是「声明不收位置参数」那一格，不是「所有命令一律拒」。
func TestCLIQ154_WaiverFace_DangerAndPassthroughUntouched(t *testing.T) {
	for _, argv := range [][]string{
		{"itask", "start", "zz-目标"}, // 危险档（D2 · `guard.go` 的 target 口）
		{"core", "reload", "zz-目标"}, // 危险档（D2）
		{"gate", "show", "zz-步名"},   // 透传型（位置参数交脚本）
	} {
		_, _, errb := helpRun(t, argv...)
		if strings.Contains(errb, "多余位置参数") {
			t.Errorf("`zerg %s` 落在豁免面（危险档/透传型）上，**不许**报「多余位置参数」：stderr=%q",
				strings.Join(argv, " "), errb)
		}
	}
}

// TestCLIQ156_UsageStringFlagsAreRealFlags —— `Q-156`：用法串里的旗标必须**真能用**（解析器认）。
// 判据值不写死：用法串从命令树真源现读（`CommandInfoOfForTest`），与 `--help` 首行逐字比；
// 那一枚旗标真给一次，不许落「未知旗标」。
func TestCLIQ156_UsageStringFlagsAreRealFlags(t *testing.T) {
	info, ok := zerg.CommandInfoOfForTest("context ls")
	if !ok {
		t.Fatal("命令树里找不到 `context ls`（判据不可判）")
	}
	if !strings.Contains(info.Usage, "--resume") {
		t.Errorf("`context ls` 的用法串要含真旗标 `--resume`（修前漏声明 ⇒ 公开面引它被判假红），得到 %q", info.Usage)
	}
	rc, out, errb := helpRun(t, "context", "ls", "--help")
	if rc != 0 {
		t.Fatalf("`zerg context ls --help` 要退 0，得到 %d · stderr=%q", rc, errb)
	}
	if got := helpFirstLine(out); got != strings.TrimSpace(info.Usage) {
		t.Errorf("`--help` 首行要逐字 = 该条用法串 %q，得到 %q", strings.TrimSpace(info.Usage), got)
	}
	if _, _, errb2 := helpRun(t, "context", "ls", "--resume"); strings.Contains(errb2, "未知旗标") {
		t.Errorf("`--resume` 是**真旗标**（`readonly.go` 的续做面）不许落「未知旗标」：stderr=%q", errb2)
	}
}

// TestCLIQ156_NegativeControl_UnknownFlagStillRefused —— 成对负控：**没写在用法串里的**旗标照旧退 2
// （补了一枚真旗标，不许把未知旗标那一格一起松掉）。
func TestCLIQ156_NegativeControl_UnknownFlagStillRefused(t *testing.T) {
	rc, _, errb := helpRun(t, "context", "ls", "--nosuchflag-zz")
	if rc != 2 || !strings.Contains(errb, "未知旗标") {
		t.Errorf("`zerg context ls --nosuchflag-zz` 要退 2 + 「未知旗标」，得到 rc=%d · stderr=%q", rc, errb)
	}
}

// TestCLIQ149_FamilyHelpHasExit —— 正控：`zerg help <族>` 退 0，且列出该族下**全部动作**的用法行
// （内容真源 = 命令树 ⇒ 用法行逐字同 `--help`）。
func TestCLIQ149_FamilyHelpHasExit(t *testing.T) {
	for _, c := range []struct{ family, wantLine string }{
		{"model", "zerg model show <模型 id> [--json <字段>]"},
		{"calib", "zerg calib ls [--json <字段>]"},
	} {
		rc, out, errb := helpRun(t, "help", c.family)
		if rc != 0 {
			t.Errorf("`zerg help %s` ⇒ 族级帮助要有出口（退 0），得到 %d · stderr=%q", c.family, rc, errb)
			continue
		}
		if !strings.Contains(out, "zerg "+c.family+" —— 族级用法") {
			t.Errorf("`zerg help %s` 首部要逐字报族级用法，得到 %q", c.family, helpFirstLine(out))
		}
		if !strings.Contains(out, c.wantLine) {
			t.Errorf("`zerg help %s` 要列出成员用法行 %q：%q", c.family, c.wantLine, out)
		}
	}
}

// TestCLIQ149_NegativeControl_TwoStatesAndPrecedenceUntouched —— 成对负控三条（`Q-149` 的边界）：
//
//	① `zerg <族> --help` 的**两态一字不动**（族名仍不在册 ⇒ 退 2 · 首行 `未知命令`）；
//	② **主题表先判**：`version` 既是族名又是主题名 ⇒ 主题面赢（`help version` 出的仍是专题）；
//	③ 族名认不出 ⇒ 仍走「未知帮助主题」（退 2 · 不新增第三种错误形状）。
func TestCLIQ149_NegativeControl_TwoStatesAndPrecedenceUntouched(t *testing.T) {
	rc, _, errb := helpRun(t, "model", "--help")
	if rc != 2 || !strings.HasPrefix(helpFirstLine(errb), "zerg: 未知命令") {
		t.Errorf("`zerg model --help` 的两态不许被改（仍退 2 + 首行 `未知命令`），得到 rc=%d · stderr=%q", rc, errb)
	}
	rc, out, errb := helpRun(t, "help", "version")
	if rc != 0 || strings.Contains(out, "族级用法") {
		t.Errorf("`zerg help version` 要走**主题表**（`version` 是既有主题名），得到 rc=%d · stdout=%q · stderr=%q",
			rc, helpFirstLine(out), errb)
	}
	rc, _, errb = helpRun(t, "help", "zz-没有这个族-zz")
	if rc != 2 || !strings.HasPrefix(helpFirstLine(errb), "zerg: 未知帮助主题") {
		t.Errorf("认不出的名字要照旧退 2 + 首行 `未知帮助主题`（不新增第三种形状），得到 rc=%d · stderr=%q", rc, errb)
	}
}

// TestCLIQ149_NegativeControl_TreeCountUnchanged —— `Q-149` 只**加出口**、不**加命令**：
// 命令树计数仍是 **119**（本批不动那一格 · 与门⑪ `T1` 的同一份投影）。
func TestCLIQ149_NegativeControl_TreeCountUnchanged(t *testing.T) {
	if n := len(zerg.CommandPathsForTest()); n != 119 {
		t.Errorf("命令树计数要仍是 119（`zerg help <族>` 不是一条新命令），得到 %d", n)
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
