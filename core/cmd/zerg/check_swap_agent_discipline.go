// check_swap_agent_discipline.go —— 「换件纪律句」内容级机检门（缺口 `GAP-20260926-193` · 甲案）。
//
// 病（缺口逐字）：换件面那句**启动方式口径**（一切执行体一律 `bash <绝对路径>` 启动 ⇒ 不依赖 x 位）
// 被反转 / 抹掉时，此前**任何门都不报** —— 按**件名**（`zerg-swap-agent.sh`）判、不按**内容**判。
// `publish/mirror-public-lib.py` 的 `--registry-parity` 里**已有**内容级守卫
// （`SWAP_AGENT_DISCIPLINE_REQUIRED/FORBIDDEN`），但那一档**没有任何门步跑它**（253 条 add_step
// 无一引 `--registry-parity`）⇒ 缺口真：守卫在盘、门户未接。
//
// 本件（薄件 · Go 半 · 无 py 半）：把那条守卫落成一道**可挂门**的判据 —— 读纪律句的**当前落点**：
//
//	· 正向句（`bash <绝对路径>` ∧ `不依赖 x 位`）在 ∧ 反转句（`⇒ **依赖 x 位**`）不在 ⇒ 绿（rc=0）；
//	· 缺正向句 或 现反转句 ⇒ 红（rc=1）；
//	· 两半皆缺（Go 半与旧壳件都不在盘）⇒ **不给结论**（rc=2 · ✗ 不静默绿）。
//
// 落点（两缀分离 · 与 py 守卫同结构 · 同族先例 `check_approve_closedset.go:79-87` 的 `cacsSyncRetired`）：
// 旧壳件 `scripts/build/zerg-swap-agent.sh` 已 Go 真搬（退役）⇒ 纪律句随落点迁到 Go 半
// `core/cmd/zerg/swap_agent.go` 的 `usage` 正文（**同一句 · 逐字在**）⇒ 退役态改校验 Go 半；
// 件在盘时校验件本体（旧路照旧）；两半皆缺 ⇒ fail-closed rc=2。
//
// 边界：只读（`os.Stat` + `os.ReadFile`）· 零子进程 · 零写盘 · 零网络 · 零状态目录；✗ 不入
// `main.go` 命令树；入 `gate_inproc_table.go` 键 `check-swap-agent-discipline`（进程内路由）。
// ✗ 不改被判对象 `core/cmd/zerg/swap_agent.go` 本体；本件判据真源 = 那段正文本身（与 py 守卫同读一句）。
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// csadShellRel —— 纪律句的**旧落点**（原壳件 · 已退役 ⇒ 只作「两缀分离」的「件」那一缀）。
const csadShellRel = "scripts/build/zerg-swap-agent.sh"

// csadGoHalfRel —— 纪律句的**当前落点**（Go 真搬件 · `zerg swap agent` 的 `usage` 正文）。
const csadGoHalfRel = "core/cmd/zerg/swap_agent.go"

// csadRequired —— 正向纪律句的**必在**子串（逐字取自 `publish/mirror-public-lib.py` 的
// `SWAP_AGENT_DISCIPLINE_REQUIRED` ⇒ 单一真源，两处同读）。
var csadRequired = []string{"bash <绝对路径>", "不依赖 x 位"}

// csadForbidden —— 反转形态的**必无**子串（逐字取自同器 `SWAP_AGENT_DISCIPLINE_FORBIDDEN`）。
var csadForbidden = []string{"⇒ **依赖 x 位**"}

// csadExists —— 普通件存在判（目录不算件）。
func csadExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

// csadRepoRoot —— 仓根：优先现读门旋钮 `ZERG_REPO_ROOT` / `ZERG_REPO`（inproc 分派会 chdir 到仓根并
// 钉这两个变量）；缺省取 cwd。取不到 ⇒ 返回空串（由调用方 fail-closed）。
func csadRepoRoot() string {
	for _, k := range []string{"ZERG_REPO_ROOT", "ZERG_REPO"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return wd
}

// csadReadSubject —— 读纪律句的当前落点，返回 (落点相对路径, 正文, state)。
// state ∈ {"ok", "unreadable"}；`unreadable` = **两半皆缺** ⇒ 调用方不给结论（rc=2）。
//
// 退役容忍（照 `publish/mirror-public-lib.py` 的守卫结构：`swap_retired` 那段 · 同族缺口
// `GAP-20261007-16`）：件在盘 ⇒ 校验件本体（旧路照旧）；件缺 ∧ Go 半在盘 ⇒ 改读 Go 半
// （**同一句 · 逐字在** ⇒ 只换真源、不拆断言）；两半皆缺 ⇒ unreadable（真·缺件 ≠ 退役）。
func csadReadSubject(root string) (string, string, string) {
	shell := filepath.Join(root, filepath.FromSlash(csadShellRel))
	gohalf := filepath.Join(root, filepath.FromSlash(csadGoHalfRel))
	retired := !csadExists(shell) && csadExists(gohalf)
	read, rel := shell, csadShellRel
	if retired {
		read, rel = gohalf, csadGoHalfRel // 退役容忍：件缺 ⇒ 改读其 Go 半
	}
	if !csadExists(read) {
		return rel, "", "unreadable" // 两半皆缺 ⇒ 缺件 ≠ 绿
	}
	b, err := os.ReadFile(read)
	if err != nil {
		return rel, "", "unreadable"
	}
	return rel, string(b), "ok"
}

// csadJudgeText —— 内容级判定（自检与真目标**共用同一套**）：返回问题清单（空 = 绿）。
func csadJudgeText(text string) []string {
	problems := []string{}
	for _, s := range csadRequired {
		if !strings.Contains(text, s) {
			problems = append(problems, fmt.Sprintf("缺正向纪律句：%q（必须在 ⇒ 被抹掉即判红）", s))
		}
	}
	for _, s := range csadForbidden {
		if strings.Contains(text, s) {
			problems = append(problems, fmt.Sprintf("出现反转句：%q（纪律句被反转 ⇒ 必报）", s))
		}
	}
	return problems
}

// csadAnyHas —— 问题清单里有没有含某子串的条目（自检用）。
func csadAnyHas(ss []string, needle string) bool {
	for _, s := range ss {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}

// csadJudgedText —— 现读落点 + 判定，返回 (rc, 问题清单, 落点相对路径, state)。
// 与自检**共用** csadReadSubject / csadJudgeText ⇒ 判据只在真目标上跑一次。
func csadJudgedText(root string) (int, []string, string, string) {
	rel, text, state := csadReadSubject(root)
	if state != "ok" {
		return 2, nil, rel, state // 两半皆缺 ⇒ 不给结论
	}
	problems := csadJudgeText(text)
	if len(problems) > 0 {
		return 1, problems, rel, state
	}
	return 0, nil, rel, state
}

// csadSelfTest —— 自检（正控 ＋ 三负控 · 纯内存 + 一枚合成仓根 · 不碰真件目录外的任何字节）。
// 末行 `自检 PASS/FAIL（N 格）`；失败格逐条打 `✗ 格N …`。
func csadSelfTest(stdout io.Writer) int {
	ok := true
	n := 0
	chk := func(label string, pass bool) {
		n++
		if !pass {
			fmt.Fprintf(stdout, "  ✗ 格%d %s\n", n, label)
			ok = false
		}
	}

	// 正面样本（逐字＝现读真句形态）：正向句在 ∧ 无反转。
	good := "  一切执行体一律 `bash <绝对路径>` 启动 ⇒ **不依赖 x 位**（真机上件可能没有 x 位）"

	// 格1 正控：正面样本 ⇒ 不判红。
	chk("正控·正面样本（正向句在 ∧ 无反转）⇒ 不判红", len(csadJudgeText(good)) == 0)

	// 格2 负控：摘掉正向句 ⇒ 判红且点名「缺正向纪律句」。
	p1 := csadJudgeText(strings.ReplaceAll(good, "不依赖 x 位", "依赖于可执行位"))
	chk("负控·摘掉正向句 ⇒ 判红且点名「缺正向纪律句」", len(p1) > 0 && csadAnyHas(p1, "缺正向纪律句"))

	// 格3 负控：现反转句 ⇒ 判红且点名「出现反转句」。
	p2 := csadJudgeText(good + "  ⇒ **依赖 x 位**")
	chk("负控·现反转句 ⇒ 判红且点名「出现反转句」", len(p2) > 0 && csadAnyHas(p2, "出现反转句"))

	// 格4 负控（任务口径②）：**两半皆缺**（空夹具仓根）⇒ state=unreadable（rc=2 不给结论，✗ 不静默绿）。
	tmp, err := os.MkdirTemp("", "csad-")
	if err != nil {
		fmt.Fprintf(stdout, "  ✗ 自检前置：夹具建不起来（%s）⇒ 自检跑不起来\n", err.Error())
		return 1
	}
	defer os.RemoveAll(tmp)
	_, _, st4 := csadReadSubject(tmp)
	chk("负控·两半皆缺（空夹具仓根）⇒ state=unreadable（rc=2 · ✗ 不静默绿）", st4 == "unreadable")

	// 格5/6 正控：真目标现读 —— 纪律句落点可读 ∧ 正文判绿（正向在 ∧ 反转不在）。
	root := csadRepoRoot()
	rel, text, st := csadReadSubject(root)
	chk("正控·真目标（纪律句落点 "+csadOrDash(rel)+"）可读（state=ok）", root != "" && st == "ok")
	if st == "ok" {
		chk("正控·真目标正文：正向句在 ∧ 反转句不在 ⇒ 判绿", len(csadJudgeText(text)) == 0)
	} else {
		chk("正控·真目标正文：判绿（前置不成立：落点读不到）", false)
	}

	res := "FAIL"
	if ok {
		res = "PASS"
	}
	fmt.Fprintf(stdout, "自检 %s（%d 格）\n", res, n)
	if ok {
		return 0
	}
	return 1
}

// csadOrDash —— 空串渲染成「—」（人面可读）。
func csadOrDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// csadUsage —— 本门用法（✗ 不入 main.go 命令树 · 仅供 --help 与拒执指路）。
const csadUsage = "usage: check-swap-agent-discipline [--root <仓根>] [--self-test] [--help]\n" +
	"  无旗标       读纪律句当前落点（Go 半 `core/cmd/zerg/swap_agent.go`）出判决\n" +
	"  --root <仓根>  显式指定仓根（缺省 = ZERG_REPO_ROOT/ZERG_REPO > cwd）\n" +
	"  --self-test   自检（正控 + 三负控：摘掉正句 / 现反转句 / 两半皆缺 ⇒ rc=2）\n" +
	"退码（逐字照 py 守卫那一路）：0 正向在 ∧ 反转不在 · 1 缺正句或现反转 · 2 两半皆缺（不给结论）\n"

// checkSwapAgentDisciplineGateMain —— 进程内入口（`gate_inproc_table.go` 键 `check-swap-agent-discipline`）。
// 退码：0 绿 · 1 红 · 2 不给结论 / 用法错。
func checkSwapAgentDisciplineGateMain(stdout, stderr io.Writer, argv []string) int {
	root := ""
	selfTest := false
	i := 0
	for i < len(argv) {
		a := argv[i]
		switch {
		case a == "-h" || a == "--help":
			fmt.Fprint(stdout, csadUsage)
			return 0
		case a == "--self-test":
			selfTest = true
		case a == "--root":
			if i+1 >= len(argv) {
				fmt.Fprintf(stderr, "check-swap-agent-discipline: 参数 %s 缺值 ⇒ rc=2\n%s", a, csadUsage)
				return 2
			}
			i++
			root = argv[i]
		case strings.HasPrefix(a, "--root="):
			root = strings.TrimPrefix(a, "--root=")
		default:
			fmt.Fprintf(stderr, "check-swap-agent-discipline: error: unrecognized arguments: %s\n%s", a, csadUsage)
			return 2
		}
		i++
	}
	if root == "" {
		root = csadRepoRoot()
	}
	if root == "" {
		fmt.Fprintf(stderr, "check-swap-agent-discipline: 解析不到仓根 ⇒ 不给结论（rc=2）\n")
		return 2
	}

	if selfTest {
		return csadSelfTest(stdout)
	}

	rc, problems, rel, state := csadJudgedText(root)
	fmt.Fprintf(stdout, "── 换件纪律句守卫（GAP-20260926-193）· 落点 %s\n", rel)
	switch state {
	case "ok":
	default:
		fmt.Fprintf(stdout, "   ⛔ 未取数：两半皆缺（`%s` 与 `%s` 都不在盘）⇒ **不给结论**（rc=2）—— 缺件 ≠ 绿\n",
			csadShellRel, csadGoHalfRel)
		return rc
	}
	fmt.Fprintf(stdout, "   正向句（%s）%s · 反转句（%s）%s\n",
		strings.Join(csadRequired, " / "),
		csadHitWord(rc == 0),
		strings.Join(csadForbidden, " / "),
		csadMissWord(rc == 0))
	if rc == 1 {
		for _, p := range problems {
			fmt.Fprintf(stdout, "   ✗ %s\n", p)
		}
		fmt.Fprintf(stdout, "⇒ 红（rc=1）：纪律句被抹掉 / 被反转 ⇒ 必报（✗ 不静默绿）\n")
		return 1
	}
	fmt.Fprintf(stdout, "⇒ 绿（rc=0）：正向句在 ∧ 反转句不在 ⇒ 启动方式口径完好\n")
	return 0
}

// csadHitWord / csadMissWord —— 正面 / 反转两栏的可读词（绿/红两态）。
func csadHitWord(okd bool) string {
	if okd {
		return "命中 ✓"
	}
	return "✗"
}

func csadMissWord(okd bool) string {
	if okd {
		return "未出现 ✓"
	}
	return "✗"
}
