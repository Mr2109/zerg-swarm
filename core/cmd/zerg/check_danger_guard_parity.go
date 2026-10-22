// check_danger_guard_parity.go —— 危险档 ↔ 执行门**对账闸**（缺口 `GAP-20260927-347` P0）。
//
// 病根（缺口原文）：`opened` 是一枚**手填静态字段**（`main.go` 的 `openedForRun`：危险档直接
// `return c.opened`），帮助面标记 / 表尾计数 / 机读面三处同源同字段 ⇒ 字段与实现一脱钩，
// 三处齐错且无任何自检能抓（缺口 345 的机制根因）。
//
// 本闸判据（可机检 · 全静态 · 零网络 · 零写盘）：
//
//	凡 `catalog()` 里 `danger != nil` 且 `!openedForRun(c)` 的命令，其 `run` **必须**走
//	未开放的统一实现 `cmdGuarded`，**或**走一枚其真跑分支内部转到 `cmdGuarded` 的包装
//	函数（白名单见 `dangerGuardParityGuardedRunners`，逐条带件:行号）。否则判红 —— 说明
//	「声明未开放」与「实现真跑」两件事脱钩：要么该命令其实有真实现（应逐条标 `opened: true`），
//	要么它该走 `cmdGuarded`（未开放）。
//
// 为什么必须合取：单看「danger 非空」或单看「run == cmdGuarded」都能骗过 ——
// 前者漏「未开放却接了真 runner」、后者漏「开放了却没登记 danger」。本闸把两者**合取**。
//
// 边界（照实）：本件**已接线（inproc 表分派）**（与 M2-0 各 Go 真搬件同规）—— 不作为 `main.go`
// 的命令、不改任何既有函数行为；接线（`add_step` / `scripts/gates`）是另一批的事（门脚本在
// `scripts/gates/**`，本轮为禁区）。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"runtime"
	"strings"
)

const dangerGuardParitySchema = "zerg-gate/1"
const dangerGuardParityKind = "DangerGuardParity"

// dangerGuardParityOffender —— 一条脱钩（证据落现读四格：命令路径 / 档 / opened / run 名）。
type dangerGuardParityOffender struct {
	Command  string `json:"command"`
	Level    string `json:"level"`
	Opened   bool   `json:"opened"`
	Run      string `json:"run"`
	Want     string `json:"want"`
	Evidence string `json:"evidence"`
}

// dangerGuardFuncName —— 取一枚 handler 的函数名（现读 · 不写死字符串）。
func dangerGuardFuncName(f func(*invocation, io.Writer, io.Writer) int) string {
	if f == nil {
		return "nil"
	}
	full := runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
	if i := strings.LastIndex(full, "."); i >= 0 {
		return full[i+1:]
	}
	return full
}

// dangerGuardParityGuardedRunners —— 「未开放的危险档」可接受的 `run` 名字集合。
//
// 为什么不是「恰好 == cmdGuarded」：本仓有四枚**包装函数**（`cmdAgentReap` ·
// `cmdDevBuild` · `cmdDevTest`），它们的**真跑分支内部才转到 `cmdGuarded`**
// （`family_reap.go:245-249` · `family_dev_candidate.go:307-341` 的 `:340`；`dev test` 只有
// `--pkg` 作用域档真起 `go test`，见 `family_dev_scoped.go:108-152`），契约真源同向
// （`core/internal/contract/dev-candidate.json:26` 的 `not_opened` · `dryrun-semantics.json:251/260`）。
// 只认函数值相等 ⇒ 这三条被判脱钩 = **假阳性**（2026-10-02 现读源码定案并纠正）。
//
// ★ 这是**白名单、不是放宽**：任何**其它** `run`（既非 `cmdGuarded`、也不在此表里）仍判红 ——
// 「声明未开放却接了真 runner」这条牙照旧在。新增包装函数必须同批登记于此（带件:行号）。
var dangerGuardParityGuardedRunners = []string{
	"cmdGuarded",         // 未开放的统一实现：只出计划件 + 拒执退码 2
	"cmdAgentReap",       // 包装：真跑分支 → cmdGuarded（family_reap.go:245-249）
	"cmdDevBuild",        // 包装：devOrderedAction → cmdGuarded（family_dev_candidate.go:282 经 :340）
	"cmdDevTest",         // 包装：候选区档同上；仅 `--pkg` 作用域档真执行（family_dev_scoped.go:108-152）
	"cmdAgentBootstrap",  // 包装：真跑分支 → cmdGuarded（family_agent_bootstrap.go · GAP-20260925-22）
	"cmdUpdateOrGuarded", // 包装：有 `--check` 走只读 `cmdUpdateCheck`，否则**原样交给** `cmdGuarded`（update_check.go:57-62 · Q-129）
}

// dangerGuardParityOffenders —— 现算脱钩清单（`catalog()` 现读 ⇒ 命令树一变它即变）。
func dangerGuardParityOffenders() []dangerGuardParityOffender {
	return dangerGuardParityOffendersIn(catalog())
}

// dangerGuardParityOffendersIn —— 现算**给定命令集**的脱钩清单（生产入口仍读 `catalog()`）。
//
// ★ 输入面显式化（`--self-test` 面所需）：判定**逐字不变**（`c.danger != nil && !openedForRun(c)`
// 且 `run` 名不在白名单里），只把「命令来源」从写死的 `catalog()` 提成入参 —— 自检据此喂合成夹具。
// 同型先例：`check_write_face_gate.go` 的 `cwfgCatalogEntries(mainGo, consts)` ·
// `check_contract_registry.go` 的 `ccrCheck(root, rootOK, reg)`（都把真源提成入参、判定一字不动）。
func dangerGuardParityOffendersIn(cmds []*command) []dangerGuardParityOffender {
	guarded := map[string]bool{}
	for _, n := range dangerGuardParityGuardedRunners {
		guarded[n] = true
	}
	want := strings.Join(dangerGuardParityGuardedRunners, " | ")
	var out []dangerGuardParityOffender
	for _, c := range cmds {
		if c.danger == nil || openedForRun(c) {
			continue
		}
		got := dangerGuardFuncName(c.run)
		if guarded[got] {
			continue
		}
		out = append(out, dangerGuardParityOffender{
			Command:  "zerg " + strings.Join(c.path, " "),
			Level:    c.danger.Level,
			Opened:   c.opened,
			Run:      got,
			Want:     want,
			Evidence: "core/cmd/zerg/main.go（命令树 path/danger/opened/run 四格现读）",
		})
	}
	return out
}

// dangerGuardParitySelfTestRunner —— `--self-test` 专用 runner 桩：函数名**不在**白名单里
// ⇒ 合成 {danger 非空 · 未开放 · run=本桩} 必判脱钩（负控的牙）。名含 SelfTest ⇒ 夹具函数
// （照 `check_write_face_gate.go` 的夹具命名规；本桩只返回 0、不写盘 ⇒ 不进任何裸写点判据）。
func dangerGuardParitySelfTestRunner(_ *invocation, _, _ io.Writer) int { return 0 }

// dangerGuardParitySelfTest —— 同型自检面（成对正负控 · 合成夹具 · 不碰真目标）。
//
// 形态逐字对齐同族先例（`check_write_face_gate.go:743` 的 `cwfgSelfTest` ·
// `check_write_authz.go:505` 的 `cwaSelfTest` · `check_gate_point_output.go:354` 的 `cgpoSelfTest`）：
// 逐格断言 · 失败打 `✗ 格%d …` · 末行 `自检 PASS/FAIL（N 格）` · rc 0 全过 / 1 有格未过。
//
// ★ fail-closed（✗ 两半皆缺仍 fail-closed）：正控（真表 · 非危险档 · 已开放 · 白名单 run）与
// 负控（合成脱钩件**必被点名**）**成对**断言 —— 任一半不出预期读数即整件 FAIL（rc≠0），
// 禁「没检到就绿」。✗ 判据一格不放宽、✗ 白名单原样。
func dangerGuardParitySelfTest(stdout io.Writer) int {
	ok := true
	n := 0
	check := func(label string, got, want int) {
		n++
		if got != want {
			fmt.Fprintf(stdout, "  ✗ 格%d %s：得 %d（要 %d）\n", n, label, got, want)
			ok = false
		}
	}
	mk := func(path []string, danger, opened bool, run func(*invocation, io.Writer, io.Writer) int) *command {
		c := &command{path: path, opened: opened, run: run}
		if danger {
			c.danger = &dangerSpec{Level: dangerD2}
		}
		return c
	}
	// 格1 正控：真表（现读 catalog()）—— 未开放的危险档 run 皆白名单 ⇒ 脱钩 0 条。
	check("正控·真表（catalog() 现读）脱钩 0 条", len(dangerGuardParityOffenders()), 0)
	// 格2 负控（牙）：合成 {danger 非空 · 未开放 · run 非白名单} ⇒ 必判 1 条。
	check("负控·合成 {danger · 未开放 · run 非白名单} 判 1 条", len(dangerGuardParityOffendersIn([]*command{
		mk([]string{"合成", "dehook"}, true, false, dangerGuardParitySelfTestRunner),
	})), 1)
	// 格3 正控：同件但**已开放**（opened=true）⇒ 不判。
	check("正控·已开放（opened）不判", len(dangerGuardParityOffendersIn([]*command{
		mk([]string{"合成", "opened"}, true, true, dangerGuardParitySelfTestRunner),
	})), 0)
	// 格4 正控：**非危险档**（danger=nil）⇒ 不判。
	check("正控·非危险档（danger=nil）不判", len(dangerGuardParityOffendersIn([]*command{
		mk([]string{"合成", "safe"}, false, false, dangerGuardParitySelfTestRunner),
	})), 0)
	// 格5 正控：白名单 runner（cmdGuarded）⇒ 不判。
	check("正控·白名单 run（cmdGuarded）不判", len(dangerGuardParityOffendersIn([]*command{
		mk([]string{"合成", "guarded"}, true, false, cmdGuarded),
	})), 0)
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

// dangerGuardParityMain —— 入口（与其它 Go 真搬门同形：rc 0 全过 / 1 有红 / 2 不给结论）。
func dangerGuardParityMain(stdout, stderr io.Writer, argv []string) int {
	wantJSON := false
	selfTest := false
	for _, a := range argv {
		if a == "--json" {
			wantJSON = true
			continue
		}
		if a == "--self-test" {
			selfTest = true
			continue
		}
		fmt.Fprintf(stderr, "usage: zerg-gate danger-guard-parity [--json] [--self-test]\n")
		return 2
	}
	if selfTest {
		return dangerGuardParitySelfTest(stdout)
	}
	iffs := dangerGuardParityOffenders()
	if wantJSON {
		obj := struct {
			Schema string                      `json:"schema"`
			Kind   string                      `json:"kind"`
			Items  []dangerGuardParityOffender `json:"items"`
		}{Schema: dangerGuardParitySchema, Kind: dangerGuardParityKind, Items: iffs}
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		_ = enc.Encode(obj)
	} else {
		for _, o := range iffs {
			fmt.Fprintf(stdout, "  ✗ %s（%s · opened=%v）：run=%s，应为 %s —— 声明未开放却接了真 runner\n",
				o.Command, o.Level, o.Opened, o.Run, o.Want)
		}
	}
	if len(iffs) > 0 {
		fmt.Fprintf(stderr, "⇒ 危险档↔执行门 脱钩 %d 条（rc=1）：danger 非空且 opened 为假，但 run≠cmdGuarded\n", len(iffs))
		return 1
	}
	fmt.Fprintf(stdout, "⇒ 危险档↔执行门 对账全过（rc=0）：凡 danger 非空且未开放者，run 皆是 cmdGuarded\n")
	return 0
}
