// cli_help_all_test.go —— `zerg help --all` 的**成对判据**（波11 序93 · 组4 §二.1 `W-08` ·
// 缺口 `Q-071`（`研-命:48`）· 同面 `B6`（`研-归:61`）· 账 `G-54`（`Q-127`））。
//
// 事项（任务单 §一 序93 逐字）：**`help --all` 补齐危险动作**（源件：`zerg help` 只列已开放的一批）。
// 判据（任务单该行「判据」栏逐字 · 口径**按现读重定**）：**`help --all` 的条数 = `help` 条数 + 危险档
// 条数**；负控：**`--all` 与 `help` 逐字不同**。
//
// 条数的**唯一**读法（与 `--- 现读` 栏同一条命令形状）：一行只要以「两个空格 + `zerg `」开头就
// 算一条；三边的数都**现跑**取（`help` / `help --all` / `help dangerous`），**一个数字都不写死** ——
// 命令面长一条，这条判据跟着长，不会变成陈旧常量（本仓 `Q-155` 同族那条教训）。
//
// 本条钉五格（两正控 + 两负控 + 一条指路）：
//
//	① 正控 · 条数等式：`--all` 的条数 == `help` 的条数 + `help dangerous` 的条数（危险档条数 > 0）；
//	② 正控 · 逐条覆盖：`help dangerous` 点名的**每一条**，在 `--all` 里都有自己那一行（用法行逐字）；
//	③ 负控 A（**修前病态**）：拿「`help` 自己的输出」冒充 `--all` 的输出（修前现读逐字「同数 ⇒
//	   `--all` 无效果 ✗」）⇒ 判据本体**必须判红**（证明它不是恒绿空转）；
//	④ 负控 B：把 `--all` 里**抹掉一条**危险动作的行 ⇒ 判据本体**必须判红**（少一条也抓得住）；
//	⑤ 对界：默认面（裸 `help` / `--help`）**一条危险动作都不列**（`--all` 是唯一出口 ⇒ 默认面
//	   没被顺手改坏）· 指路句在两种面上都在（可发现性不是靠猜）。
//
// 落点：`package main_test` ＋ `zerg.RunForTest` —— 跑的是**当前源码**的行为（不是盘上旧制品），
// 与 `cli_usage_nextstep_test.go` / `cli_help_twostate_test.go` 同一条路；全程只读，
// 不起子进程、不碰状态目录、不写任何件。
package main_test

import (
	"regexp"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// helpLinePrefix —— 「算一条」的行首（与任务单现读栏的 grep 形状逐字同：两个空格 + `zerg `）。
const helpLinePrefix = "  zerg "

// helpBadgeLine —— `--all` 的**指路句**（两种面上都必须有；它自己不以 `helpLinePrefix` 开头 ⇒
// 不进条数等式 —— 这条是「等式不被指路句污染」的守卫）。
const helpBadgeMark = "`zerg help --all`"

// countZergLines 数「两个空格 + zerg 」开头的行（三边同形状 ⇒ 可比）。
func countZergLines(s string) int {
	n := 0
	for _, ln := range strings.Split(s, "\n") {
		if strings.HasPrefix(ln, helpLinePrefix) {
			n++
		}
	}
	return n
}

// dangerNames 从 `help dangerous` 的现跑输出里抽**命令名**（逐条）。
// 抽法 = `zerg` 之后**连续的小写词**（命令树里每一段都是小写）—— 遇到旗标/占位符（`-` `<` `[`）
// 或档位标记（`D2` / `D3`）自然停；`script inventory sync` 这种三段名也抽得全。
// 与 `check-public-face-commands.py` 的 `_path_of` 同口径（那一条按「首个旗标/占位符」切，本格多切一个档位）。
var dangerNameRe = regexp.MustCompile(`^\s{2}zerg\s+((?:[a-z][a-z0-9]*)(?:\s+[a-z][a-z0-9]*)*)`)

func dangerNames(out string) []string {
	var names []string
	for _, ln := range strings.Split(out, "\n") {
		if m := dangerNameRe.FindStringSubmatch(ln); m != nil {
			names = append(names, m[1])
		}
	}
	return names
}

// helpAllHolds —— 判据本体（**唯一**一处）：`--all` 的条数 = 裸 `help` 的条数 + 危险档条数。
// 危险档条数 <= 0 时**恒判红**（没有危险档可比 ⇒ 这条等式是空转，不许给绿）。
func helpAllHolds(base, all string, dangerN int) bool {
	if dangerN <= 0 {
		return false
	}
	return countZergLines(all) == countZergLines(base)+dangerN
}

// stripDangerLine 把 `--all` 输出里**某一条**危险动作的行摘掉（负控 B 的夹具）。
func stripDangerLine(all, name string) string {
	var keep []string
	for _, ln := range strings.Split(all, "\n") {
		if strings.HasPrefix(ln, helpLinePrefix+name) {
			continue
		}
		keep = append(keep, ln)
	}
	return strings.Join(keep, "\n")
}

// helpFace 现跑一次帮助面 ⇒ (rc, stdout)（stderr 不要：这几条都不该走错误面）。
func helpFace(t *testing.T, argv ...string) (int, string) {
	t.Helper()
	var out, errb strings.Builder
	rc := zerg.RunForTest(argv, &out, &errb)
	return rc, out.String()
}

// TestCLIHelpAll_CountEquation —— ① 正控：`help --all` 的条数 = `help` 的条数 + 危险档条数。
func TestCLIHelpAll_CountEquation(t *testing.T) {
	rcB, base := helpFace(t, "help")
	rcA, all := helpFace(t, "help", "--all")
	rcD, dang := helpFace(t, "help", "dangerous")
	if rcB != 0 || rcA != 0 || rcD != 0 {
		t.Fatalf("三条现跑都要退 0（判据不可判）：help=%d · help --all=%d · help dangerous=%d", rcB, rcA, rcD)
	}
	names := dangerNames(dang)
	nB, nA, nD := countZergLines(base), countZergLines(all), len(names)
	t.Logf("现跑：help=%d 条 · help --all=%d 条 · 危险档=%d 条（等式右端应为 %d）", nB, nA, nD, nB+nD)
	if nD <= 0 {
		t.Fatalf("危险档条数现读 %d（<=0）⇒ 等式是空转，不许给绿（`help dangerous` 输出变了？）", nD)
	}
	if !helpAllHolds(base, all, nD) {
		t.Errorf("判据不成立：`help --all` %d 条 ≠ `help` %d 条 + 危险档 %d 条（= %d）· 缺口 `Q-071`",
			nA, nB, nD, nB+nD)
	}
}

// TestCLIHelpAll_CoversEveryDanger —— ② 正控：`help dangerous` 点名的每一条，在 `--all` 里都有自己那一行。
// 只核条数还不够 —— 少一条 + 多一条也能凑出等式；这一格把「是哪几条」钉住。
func TestCLIHelpAll_CoversEveryDanger(t *testing.T) {
	_, all := helpFace(t, "help", "--all")
	_, dang := helpFace(t, "help", "dangerous")
	names := dangerNames(dang)
	if len(names) == 0 {
		t.Fatal("`help dangerous` 里一条命令名都抽不出来（判据不可判）")
	}
	missing := []string{}
	for _, n := range names {
		if !strings.Contains(all, helpLinePrefix+n) {
			missing = append(missing, n)
		}
	}
	if len(missing) > 0 {
		head := missing
		if len(head) > 5 {
			head = head[:5]
		}
		t.Errorf("`--all` 漏了 %d 条危险动作（前 5 条：%s）", len(missing), strings.Join(head, " · "))
	}
}

// TestCLIHelpAll_NegativeControl_NoEffectIsRed —— ③ 负控 A：修前病态（`--all` 无效果）必须判红。
// 修前现读逐字：`help` 与 `help --all` **同数** ⇒ 拿 `help` 的输出冒充 `--all` 的输出，
// 判据本体必须判红，否则这条等式只是恒绿。
func TestCLIHelpAll_NegativeControl_NoEffectIsRed(t *testing.T) {
	_, base := helpFace(t, "help")
	_, dang := helpFace(t, "help", "dangerous")
	nD := len(dangerNames(dang))
	if nD <= 0 {
		t.Fatal("危险档条数现读 0（判据不可判）")
	}
	if helpAllHolds(base, base, nD) {
		t.Errorf("负控失败：`--all` **无效果**（整个面逐字等于 `help`）时判据仍给绿 ⇒ 判据认的不是那一件（恒绿空转）")
	}
}

// TestCLIHelpAll_NegativeControl_DroppedLineIsRed —— ④ 负控 B：`--all` 少列一条必须判红。
func TestCLIHelpAll_NegativeControl_DroppedLineIsRed(t *testing.T) {
	_, base := helpFace(t, "help")
	_, all := helpFace(t, "help", "--all")
	_, dang := helpFace(t, "help", "dangerous")
	names := dangerNames(dang)
	if len(names) == 0 {
		t.Fatal("危险档条数现读 0（判据不可判）")
	}
	if !helpAllHolds(base, all, len(names)) {
		t.Fatal("负控前置不成立：原始 `--all` 输出就过不了等式（先修正控）")
	}
	mutated := stripDangerLine(all, names[0])
	if mutated == all {
		t.Fatalf("夹具没摘到任何一行（`--all` 里找不到 `%s`）", names[0])
	}
	if helpAllHolds(base, mutated, len(names)) {
		t.Errorf("负控失败：抹掉 `%s` 那一行后判据仍给绿 ⇒ 少一条抓不住", names[0])
	}
}

// TestCLIHelpAll_DefaultFaceStaysOpenOnly —— ⑤ 对界：默认面**一条危险动作都不列**、指路句在两种面上都在。
// 修法只能是「多一个出口」，不许顺手把危险动作塞进默认面（那会让默认面与 `help dangerous` 两处打架，
// 且默认面的条数等式（`help dangerous` 的 `上表 N 条` 自洽）也会被搅）。
func TestCLIHelpAll_DefaultFaceStaysOpenOnly(t *testing.T) {
	rcB, base := helpFace(t, "help")
	rcH, viaHelp := helpFace(t, "--help")
	_, dang := helpFace(t, "help", "dangerous")
	if rcB != 0 || rcH != 0 {
		t.Fatalf("`help`=%d · `--help`=%d —— 都要退 0", rcB, rcH)
	}
	for _, n := range dangerNames(dang) {
		if strings.Contains(base, helpLinePrefix+n) {
			t.Errorf("默认面（裸 `help`）里出现了危险动作 `%s` 的行 —— 默认面只列已开放的一批", n)
		}
		if strings.Contains(viaHelp, helpLinePrefix+n) {
			t.Errorf("默认面（`--help`）里出现了危险动作 `%s` 的行 —— 默认面只列已开放的一批", n)
		}
	}
	for _, c := range []struct {
		name string
		out  string
	}{{"help", base}, {"--help", viaHelp}} {
		if !strings.Contains(c.out, helpBadgeMark) {
			t.Errorf("`zerg %s` 的面上找不到指路句 `%s`（`--all` 就不可发现）", c.name, helpBadgeMark)
		}
	}
}
