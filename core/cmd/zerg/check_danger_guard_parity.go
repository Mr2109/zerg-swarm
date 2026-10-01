// check_danger_guard_parity.go —— 危险档 ↔ 执行门**对账闸**（缺口 `GAP-20260927-347` P0）。
//
// 病根（缺口原文）：`opened` 是一枚**手填静态字段**（`main.go` 的 `openedForRun`：危险档直接
// `return c.opened`），帮助面标记 / 表尾计数 / 机读面三处同源同字段 ⇒ 字段与实现一脱钩，
// 三处齐错且无任何自检能抓（缺口 345 的机制根因）。
//
// 本闸判据（可机检 · 全静态 · 零网络 · 零写盘）：
//
//	凡 `catalog()` 里 `danger != nil` 且 `!openedForRun(c)` 的命令，其 `run` **必须是**
//	`cmdGuarded`（未开放的统一实现：只出计划件 + 拒执退码 2）。否则判红 —— 说明
//	「声明未开放」与「实现真跑」两件事脱钩：要么该命令其实有真实现（应逐条标 `opened: true`），
//	要么它该走 `cmdGuarded`（未开放）。
//
// 为什么必须合取：单看「danger 非空」或单看「run == cmdGuarded」都能骗过 ——
// 前者漏「未开放却接了真 runner」、后者漏「开放了却没登记 danger」。本闸把两者**合取**。
//
// 边界（照实）：本件**只落码、不接线**（与 M2-0 各 Go 真搬件同规）—— 不作为 `main.go`
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

// dangerGuardParityOffenders —— 现算脱钩清单（`catalog()` 现读 ⇒ 命令树一变它即变）。
func dangerGuardParityOffenders() []dangerGuardParityOffender {
	want := dangerGuardFuncName(cmdGuarded)
	var out []dangerGuardParityOffender
	for _, c := range catalog() {
		if c.danger == nil || openedForRun(c) {
			continue
		}
		got := dangerGuardFuncName(c.run)
		if got == want {
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

// dangerGuardParityMain —— 入口（与其它 Go 真搬门同形：rc 0 全过 / 1 有红 / 2 不给结论）。
func dangerGuardParityMain(stdout, stderr io.Writer, argv []string) int {
	wantJSON := false
	for _, a := range argv {
		if a == "--json" {
			wantJSON = true
			continue
		}
		fmt.Fprintf(stderr, "usage: zerg-gate danger-guard-parity [--json]\n")
		return 2
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
