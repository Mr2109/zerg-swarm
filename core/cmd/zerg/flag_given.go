// flag_given.go —— 值旗标「给没给」的**唯一判据**（given 状态位 · 缺口 `GAP-20260926-58`）。
//
// 缺口原文：「值旗标裸形的补丁已出现**三种口径**（`hasFlag` / 直读 `kv` / 重扫 `orig`），
// 无统一状态位」。本件收敛成一套 —— 收口前仓里三处各写一套（逐处现读位置见交付回报）：
//
//	① `inv.hasFlag(name)`（`main.go:5472` 的 kv 存在性原语）：只认写进 `inv.kv` 的那批。
//	   值旗标裸形它**看得见**（`parseInvocation` 的 `setValueFlag(a, "")` ⇒ 键在、值空），
//	   但**类型化字段**（`--model` / `--desc` / `--priority` / `--slice-id` / `--depends-on` /
//	   `--acceptance` / `--to` / `--set`，走 `inv.modelWant` 等专用字段、**不进 `kv`**）它恒假
//	   ⇒ 谁按它判「给了 --model」就得出「给了却说没给」（`family_agent_run.go:184` 那条血证）。
//	② 直读 `inv.kv[f]`（`family_script_inventory.go` 的写旗标拒执守卫）：与 ① 同口径，但是
//	   **第二份手写**（换了件就各自实现一遍）。
//	③ 另写一套「扫原始 argv」（`agentRunFlagGiven` / `impactBackfillFlagGiven` / `flagValue`）：
//	   能看见 ① 看不见的类型化字段，但每处一份循环（三份），且**看不见** kv 里归一过的形态。
//
// 本件 = ④：`inv.flagGiven(name)` —— **判据唯一实现**（`kv 键在 ‖ 原始 argv 有这枚旗标`）。
// 收敛动作：② 改调它；③ 的调用点改调它，`flagValue` 要的是**值**（不是「给没给」），故留作
// 值读器，但它的扫描循环同样落在本件唯一那份 `flagArgScan` 上 ⇒ 仓里只剩一份 given 判据、
// 一份 argv 扫描实现。
//
// ⚠ 边界（照实）：
//   - `inv.hasFlag` 的定义在 `main.go`（本单禁区：另一路正在写它）⇒ 它仍是一枚**独立原语**，
//     语义 = 「`inv.kv` 里有这个键」，**未**改调本件。要把这一格也一并收口，只需把
//     `main.go:5472-5475` 的体换成一行 `return inv.flagGiven(name)`（见交付回报「待改行」）。
//   - 「给过」与「给了值」是**两件事**：裸形 ⇒ `flagGiven` 真、`flagVal` 空串 —— 由各命令
//     按自己的口径判用法错 2（✗ 不在本件把两态合并）。
//   - 判据只看「这枚旗标在 argv 里出现过没有」，**不**判它是否被 `--` 隔开（与收口前
//     ③ 的三份实现逐字同口径 —— 行为不变的硬要求；这一格属已知口径，另立账再说）。
package main

import "strings"

// flagArgScan —— 「在原始 argv 里找一枚旗标」的**唯一扫描实现**（③ 那三份循环都收在这里）。
//
// 形态（与用户敲的逐字对齐）：
//
//	`--k=v`  ⇒ (v,     true,  true)
//	`--k v`  ⇒ (v,     true,  true)
//	`--k` 在尾⇒ ("",    false, true)   ← 给过、但**没有值槽**（`impactBackfillFlagGiven` 要的正是这一格）
//	没出现   ⇒ ("",    false, false)
//
// ★ 返回值三态而不是两态：`flagValue`（`export.go`）要的是「有值槽才算取到」，而 given 只要
// 「出现过」⇒ 一份扫描喂两个问法，✗ 不各写一份。
func flagArgScan(orig []string, name string) (val string, valueSlot bool, found bool) {
	for i, a := range orig {
		if a == name {
			if i+1 < len(orig) {
				return orig[i+1], true, true
			}
			return "", false, true
		}
		if strings.HasPrefix(a, name+"=") {
			return strings.TrimPrefix(a, name+"="), true, true
		}
	}
	return "", false, false
}

// flagArgGiven —— 「这枚旗标在原始 argv 里出现过没有」（`--k` 与 `--k=v` 两形态都算给）。
func flagArgGiven(orig []string, name string) bool {
	_, _, found := flagArgScan(orig, name)
	return found
}

// flagGiven —— 值旗标「给没给」的**唯一判据**（given 状态位）。
//
// 两路取证，缺一不可：
//
//	① `inv.kv` 有键 ⇒ 给过（解析面对值旗标裸形/`--k=v`/`--k v` 的**归一结果**；裸形 ⇒ 键在、值空）；
//	② 原始 argv 出现过 ⇒ 给过（覆盖 ① 看不见的**类型化字段** `--model` 一类，以及
//	   `--json` / `--confirm` / `--ttl` / `--schema` / `--node` / `--context` 这些
//	   走专用 `*Given` 字段、同样不进 `kv` 的旗标）。
//
// 反例由单测钉住（`flag_given_test.go`）：「没给」必须假 —— 值里长得像旗标（`--desc --model`）
// 不算给（那是解析面的裸形语义，不是本判据的事）。
func (inv *invocation) flagGiven(name string) bool {
	if _, ok := inv.kv[name]; ok {
		return true
	}
	return flagArgGiven(inv.orig, name)
}
