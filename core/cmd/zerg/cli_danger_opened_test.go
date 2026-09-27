// cli_danger_opened_test.go —— 「`opened` 字段」的对账闸（缺口 GAP-20260927-347 的**机制根因** · 2026-09-27）。
//
// 病（照实现读出来的）：
//
//	main.go 的 `openedForRun(c)`（危险档那一支）**直接 `return c.opened`** —— `opened` 是**手填的静态字段**，
//	不按实现现算；而它正是三处显示面的**唯一**口径：
//	  · guard.go 的逐条标记（`← 本版未开放` / `← **已开放**`）；
//	  · guard.go 的表尾计数（`上表 N 条：已开放 X / 未开放 Y`）；
//	  · export.go 的机读面（逐条格④「本版开没开」）。
//	⇒ 一条**真跑已经能执行**的危险动作，只要漏了 `opened: true` 一行，就会被三面齐声报成「本版未开放」，
//	而**没有任何判据会红** —— 表与实现静默脱钩。
//
// 现读实据（修前）：`zerg gap export <目录> --yes` 真跑 **rc=0 且真写盘**（`--out` 拆件落盘，
// 索引件 1 + 页件 N），缺 `--yes` ⇒ rc=2，`--dry-run` ⇒ rc=0 出计划件；而 `zerg help dangerous`
// 那一行仍写 `← 本版未开放`、表尾仍把它算进「未开放」。
//
// 闸的形状（**不按实现现算就不许叫「已开放」**，反过来也不许漏标）：
//
//	① 凡 `danger != nil && !opened` 的条目 ⇒ 它挂的 runner **必须在「真跑自拒」闭集里**
//	   （`refusingRunners`：通用守门 `cmdGuarded` + 三枚族内自带自拒的 runner）。
//	   落到闭集外 ⇒ 表说「本版未开放」，而实现挂着一枚**没登记会拒执**的函数 ⇒ 当场红、点名到条目。
//	   这就是 `gap export` 修前要红的那一格（它的 runner 是 `cmdGapExport`，不在闭集里）。
//	② 反向防呆：凡挂 `cmdGuarded` 的**危险档**条目 ⇒ 必须 `!opened`
//	   （`cmdGuarded` 恒拒执真跑，标了 `opened: true` 就是**标记撒谎**）。
//	③ 闭集活性：闭集里每一枚 runner 都得在命令树里**真被用上**（防闭集腐烂成空转 ⇒ 闸永远绿）。
//	④ 内建负控：判定口是**纯函数** `dangerOpenedProblems`，正控（真命令树）与负控（合成夹具）都调它 ——
//	   合成夹具必须被判红（判据不是恒绿）。
//
// 数：★ 本件**不记任何绝对条数**（件头读数会烂、且改数字=明年还会烂）——
// 危险档总条数 / 已开放 / 未开放三格一律由 `zerg help dangerous` 现读，本件只判**形状**；下面那串 runner
// 分布 =
// `cmdGuarded` 25 · `cmdAgentReap` 1 · `cmdDevBuild` 1 · `cmdDevTest` 1 · `cmdGapExport` 1（**修前这一枚就是脱钩的那条**）。
package main

import (
	"fmt"
	"io"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// refusingRunners —— **闭集**：在「真跑」那一态**自拒执行**的那些 runner。
//
// 判据（与 `openedForRun` 配套）：一条危险档命令被登记成 `opened: false`，等于对外**承诺**「真跑一律拒执、
// 退码 2 = 不给结论，只有 `--dry-run` 的计划面可用」。承诺要兑现，就只能挂下面这几枚：
//
//   - `cmdGuarded`    —— 通用守门（库内绝大多数未开放危险档走的这一枚：`--dry-run` 出计划件 0 / 真跑拒执 2）；
//   - `cmdAgentReap`  —— `agent reap`：默认干跑；真回收本版未开放（入库件永不进候选 = 红线）；
//   - `cmdDevBuild`   —— `dev build`：候选区构建，**判据先于自动化**（找不到验收证据单 ⇒ 2）；
//   - `cmdDevTest`    —— `dev test`：同上，真跑本版未开放。
//
// ★ 它**只收「确知会自拒」的那几枚**：新写一枚写面实现却忘标 `opened: true`、或反过来把已开放的 runner
// 登记成未开放 —— 两种都落在闭集外 / 撞 ② ⇒ 本件当场红并点名到条目。这就是「`opened` 不再能静默脱钩」。
var refusingRunners = []func(*invocation, io.Writer, io.Writer) int{
	cmdGuarded,
	cmdAgentReap,
	cmdDevBuild,
	cmdDevTest,
}

// runnerName —— 函数值 ⇒ 可读名（`main.cmdGuarded`）。判红时**点名到条目**用。
func runnerName(f func(*invocation, io.Writer, io.Writer) int) string {
	if f == nil {
		return "<nil>"
	}
	if fn := runtime.FuncForPC(reflect.ValueOf(f).Pointer()); fn != nil {
		return fn.Name()
	}
	return fmt.Sprintf("func@%#x", reflect.ValueOf(f).Pointer())
}

// dangerOpenedProblems —— **判定口**（纯函数 · 正控与负控都调它 ⇒ 判据不是恒绿）。
// 返回每一条「表 ↔ 实现脱钩」的判词；空切片 = 合。
func dangerOpenedProblems(cmds []*command) []string {
	var probs []string
	// ★ 键用**函数名**（`runtime.FuncForPC`），不用 `reflect` 的代码指针：
	//   实测（2026-09-27）包级函数取 `Pointer()` 在**内联**等情形下不保证唯一 ——
	//   `cmdGuarded` 碰巧匹配得上，而 `cmdAgentReap` / `cmdDevBuild` / `cmdDevTest` 三枚
	//   明明在命令树里（main.go:1422/1603/1612）却被判「一枚都没用上 ⇒ 闭集腐烂」。
	//   按名字比是**编译期确定**的（函数名不随内联/包装漂），且与 `runnerName` 点名同一口径。
	refuse := map[string]bool{}
	for _, f := range refusingRunners {
		refuse[runnerName(f)] = true
	}
	for _, c := range cmds {
		if c.danger == nil {
			continue
		}
		name := "zerg " + strings.Join(c.path, " ")
		if !c.opened {
			if !refuse[runnerName(c.run)] {
				probs = append(probs, fmt.Sprintf(
					"%s：登记 `opened: false`（危险档「本版未开放」）却挂着 `%s` —— 它不在「真跑自拒」闭集里 ⇒ **表与实现脱钩**（要么这条其实已开放 ⇒ 补一行 `opened:   true,`；要么真跑确实没拒执 ⇒ 补实现或改登记）",
					name, runnerName(c.run)))
			}
			continue
		}
		if runnerName(c.run) == runnerName(cmdGuarded) {
			probs = append(probs, fmt.Sprintf(
				"%s：标了 `opened: true`（对外承诺「确认档齐就真执行」）却挂着恒拒执真跑的 `cmdGuarded` ⇒ **标记撒谎**（撤标记，或换上有真实现的 runner）", name))
		}
	}
	return probs
}

// refusingRunnersLiveness —— ③ 闭集活性（**只对真命令树跑**）：闭集里每一枚 runner 都得在树里真被用上
// （防闭集腐烂成空转 ⇒ 闸永远绿）。
//
// ★ 为什么**不能**并进 `dangerOpenedProblems`：那个判定口是**纯函数**，正控（真树）与负控（只含一两条的
//
//	合成夹具）都调它。夹具天生不含全量命令 ⇒ 活性检查在夹具上必然误报「闭集腐烂」，把负控③（合的形状
//	不许判红）直接顶红。实测（2026-09-27）：修 `runnerPtr` ⇒ `runnerName` 之后三枚在用的 runner
//	（`cmdAgentReap`/`cmdDevBuild`/`cmdDevTest`）才真正被认出来，原先 uintptr 比法恰好看不出这个问题 ——
//	**那说明活性检查此前一直在空转**，不是它对了。
func refusingRunnersLiveness(cmds []*command) []string {
	var probs []string
	used := map[string]bool{}
	for _, c := range cmds {
		if c.run != nil {
			used[runnerName(c.run)] = true
		}
	}
	for _, f := range refusingRunners {
		if !used[runnerName(f)] {
			probs = append(probs, fmt.Sprintf(
				"闭集里的 `%s` 在命令树里一枚都没用上 ⇒ 闭集腐烂（这条闸正在空转：删掉它，或补上它的消费者）", runnerName(f)))
		}
	}
	return probs
}

// TestDangerOpenedFace —— ① / ② / ③ 正控：拿**真命令树**（`catalog()`，与三处显示面同源）过判定口 ⇒ 必须空。
func TestDangerOpenedFace(t *testing.T) {
	cmds := catalog()
	if len(cmds) == 0 {
		t.Fatal("命令树是空的 ⇒ 判据不可判（不给绿）")
	}
	nDanger, nOpen := 0, 0
	for _, c := range cmds {
		if c.danger != nil {
			nDanger++
			if c.opened {
				nOpen++
			}
		}
	}
	if nDanger == 0 {
		t.Fatalf("命令树里一条危险档都没有（现读 %d）⇒ 本件的 ①/② 是空转，不许给绿（`danger` 登记面被改过？）", nDanger)
	}
	if probs := dangerOpenedProblems(cmds); len(probs) > 0 {
		t.Errorf("危险档 %d 条（已开放 %d / 未开放 %d）：表 ↔ 实现脱钩 %d 处 ——\n  · %s",
			nDanger, nOpen, nDanger-nOpen, len(probs), strings.Join(probs, "\n  · "))
	}
	// ③ 闭集活性：只对**真树**跑（夹具上必误报，见 refusingRunnersLiveness 的注释）。
	if probs := refusingRunnersLiveness(cmds); len(probs) > 0 {
		t.Errorf("闭集活性失守 %d 处 ——\n  · %s", len(probs), strings.Join(probs, "\n  · "))
	}
}

// TestDangerOpenedFace_NegativeControl —— ④ 内建负控：合成夹具**必须**被判红（判据不是恒绿）。
func TestDangerOpenedFace_NegativeControl(t *testing.T) {
	// 负控① —— 复刻 `gap export` 修前的形状：危险档 + `opened: false` + 一枚**不在闭集里**的 runner。
	// （修前真身 = `cmdGapExport`；这里用匿名函数，避免把「真身已补 opened」这件事当夹具。）
	foreign := func(*invocation, io.Writer, io.Writer) int { return 0 }
	fix1 := []*command{{
		path:   []string{"gap", "export"},
		kind:   "GapExport",
		danger: &dangerSpec{Level: dangerD2, Target: "落点目录", Effect: "拆件落盘", Source: "负控夹具"},
		run:    foreign,
	}}
	if probs := dangerOpenedProblems(fix1); len(probs) == 0 {
		t.Error("负控① 失守：危险档 `opened: false` + 闭集外的 runner **没被判红** ⇒ 判定口是恒绿的（闸空转）")
	}
	// 负控② —— 反向：`opened: true` 却挂恒拒执的 `cmdGuarded`。
	fix2 := []*command{{
		path:   []string{"x", "y"},
		kind:   "XY",
		danger: &dangerSpec{Level: dangerD2, Target: "t", Effect: "e", Source: "负控夹具"},
		opened: true,
		run:    cmdGuarded,
	}}
	if probs := dangerOpenedProblems(fix2); len(probs) == 0 {
		t.Error("负控② 失守：`opened: true` 挂 `cmdGuarded` **没被判红** ⇒ 反向防呆空转")
	}
	// 负控③ —— 合的形状（未开放挂 `cmdGuarded`）**不许**判红（防「一律红」这类假闸）。
	fix3 := []*command{{
		path:   []string{"z", "w"},
		kind:   "ZW",
		danger: &dangerSpec{Level: dangerD3, Target: "t", Effect: "e", Source: "负控夹具"},
		run:    cmdGuarded,
	}}
	if probs := dangerOpenedProblems(fix3); len(probs) > 0 {
		t.Errorf("负控③ 失守：合的形状（未开放挂 cmdGuarded）竟被判红 ⇒ 闸是「一律红」的假闸：%v", probs)
	}
	// 负控④ —— 闭集活性**不是恒绿**：拿只含一条无关命令的夹具调它 ⇒ 必须报出闭集里所有没被用上的 runner。
	//   （`fix3` 里挂了 `cmdGuarded` ⇒ 它算**被用上**，故实报数 = 闭集大小 − 1。）
	if probs := refusingRunnersLiveness(fix3); len(probs) != len(refusingRunners)-1 {
		t.Errorf("负控④ 失守：夹具只用了闭集里的 1 枚 ⇒ 该报 %d 处（闭集 %d − 1），实得 %d 处 ⇒ 它可能恒绿：%v",
			len(refusingRunners)-1, len(refusingRunners), len(probs), probs)
	}
	// 负控⑤ —— 活性检查在**真树**上必须空（否则正控会红，这条负控保证上面那格不是「一律红」）。
	if probs := refusingRunnersLiveness(catalog()); len(probs) > 0 {
		t.Errorf("负控⑤ 失守：活性检查在真命令树上竟报红 %d 处：%v", len(probs), probs)
	}
}
