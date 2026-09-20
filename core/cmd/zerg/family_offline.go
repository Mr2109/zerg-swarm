// family_offline.go —— 离线 / 降级路径（§十 M20 · §十五.4 · §十二 `P-114`–`P-117` · §7.1 `P10`）。
//
// 本件不新增命令（M20 的判据全落在**既有命令的行为**上）；它做两件事：
//
//	① 把 M20 的条文落成**一处可查的自描述面**（`zerg help offline`）：
//	   `O1`–`O8` 八条 + 例外清单 `F-1`–`F-4` + 退码面（`kind=unreachable` · `detail` 五值 · `where`）。
//	② 把**例外清单与具体命令对上**（§十二 `P-116` 的「定案」那一格）：
//	   哪些命令**允许**不经主控 —— 逐条点名，且写明 `F-4`（零网络的本机事实面）**不进**例外清单
//	   （它压根不碰网络 ⇒ 谈不上"例外"）。
//
// 为什么 `--offline` / `--prefer-cache` **先不开**（`P-115` 定案）：只读本地缓存看起来无害，
// 但它会让「这份数据是哪一刻的」失去唯一的判据 —— 契约现在只有一条口径：**打不到就是打不到**。
// 命令面因此**没有**这两枚旗标（现跑 `zerg task ls --offline` ⇒ 未知旗标 ⇒ 2）。
package main

import (
	"fmt"
	"strings"
)

// offlineExceptionRow —— 例外清单的一行（`F-*` × 命令 × 为什么能例外）。
type offlineExceptionRow struct {
	ID      string
	Command string
	Why     string
}

// offlineExceptions —— `F-1`–`F-4` 逐条与**具体命令**对上（本件就是那一处真源）。
func offlineExceptions() []offlineExceptionRow {
	return []offlineExceptionRow{
		{"F-1", "zerg agent bootstrap <机>", "子端引导：目标机上还没有子端可经主控 —— 引导面必须独立（危险档 · 本版未开放）"},
		{"F-2", "zerg agent ping <机> --direct <host:port>", "探活/诊断：**探的就是「通不通」**，经主控就测不出链路；输出必带回显 `via=direct`"},
		{"F-3", "zerg update · zerg core update", "主控自身换件/自更新：要换的正是主控 —— 它不能要求主控先可用（危险档 · 本版未开放）"},
		{"F-4", "（不得列为例外）zerg version / help / context ls / doctor --local", "零网络的本机事实面：它们**压根不碰网络** ⇒ F-4 是「不用例外」的说明，**不进**例外清单"},
	}
}

// offlineDetailValues —— 报错面（§十五.5）：`kind` 取自 M7 闭集，`detail` 只有这五值，`where` 只有 core/agent。
var offlineDetailValues = []string{"dns_failure", "connection_refused", "timeout", "tls_failure", "proxy_failure"}

// helpOffline —— `zerg help offline`（M20 的自描述面）。
func helpOffline() string {
	var b strings.Builder
	b.WriteString("离线 / 降级路径（§十 M20 · §十五.4 · 调研-M20 条文 `O1`–`O8`）\n\n")
	b.WriteString("**一条总口径**：命令面默认**只打一个端点**（档决定的那个）；\n")
	b.WriteString("**没有第二个端点会被自动尝试**（不偷偷直连子端 · 不偷偷换目标 · 不偷偷降级成本地 · 不偷偷重试）。\n\n")
	b.WriteString("八条（`O1`–`O8` · 逐条给判据）：\n")
	b.WriteString("  O1 默认单端点 —— 端点来自「档」（`context ls` 可查），来源级只有 `flag/env/config/builtin-local` 四种。\n")
	b.WriteString("  O2 四条「不偷偷」 —— 现跑可验：主控不可达时**只报一次**、不换端点、不读缓存当结果。\n")
	b.WriteString("  O3 不可达即 **fail-closed**：整条命令立即失败（不产结果、不问任何问题）。\n")
	b.WriteString("  O4 错误里**必须点名打不到的是谁** + 端点来自哪一级（`error.where` = `core|agent`）。\n")
	b.WriteString("  O5 端点来源级四值（见 O1）。\n")
	b.WriteString("  O6 不许重复报同一句（同一次调用里同一个错只打一次）。\n")
	b.WriteString("  O7 退码面：`kind=unreachable` · `detail` ∈ {" + strings.Join(offlineDetailValues, " / ") + "}（§十五.5 · M20:347）· `where` ∈ `core|agent` · 退码 `12`。\n")
	b.WriteString("  O8 离线可跑白名单**四条**：`version` · `help` · `context ls|show` · `doctor --local`。\n\n")
	b.WriteString("例外清单（`F-1`–`F-4` · **只有这四条** · 每条都要显式命令或旗标 + 输出回显 `via`）：\n")
	for _, r := range offlineExceptions() {
		fmt.Fprintf(&b, "  %-4s %-46s %s\n", r.ID, r.Command, r.Why)
	}
	b.WriteString("\n**先不开的两枚**（§十二 `P-115` 定案）：`--offline` 与 `--prefer-cache`。\n")
	b.WriteString("  理由：只读本地缓存会让「这份数据是哪一刻的」失去唯一判据 —— 契约现在只有一条：\n")
	b.WriteString("  **打不到就是打不到**（现跑 `zerg task ls --offline` ⇒ 未知旗标 ⇒ 退码 2）。\n\n")
	b.WriteString("`doctor` 在主控不可达时的默认面（§十二 `P-116` 定案）：\n")
	b.WriteString("  · 本机项照旧判（仓根 · 门禁入口 · 凭据面）；**主控可达**与**子端名册**两项判 `BLOCKED`；\n")
	b.WriteString("  · 整单退码取**最重**那一格 ⇒ 有 `BLOCKED` ⇒ `8`（「读不到」**不许**当健康 · §九 M9 `RC9`）；\n")
	b.WriteString("  · **不**因为打不到主控就退回「只看本机」的一副面孔 —— 两张脸会让「健康」这个词在两个语境里不同义。\n")
	b.WriteString("\n【口径差 · 照实记（本命令面现跑 vs §十五.5 声明）】\n")
	b.WriteString("  · `detail`：契约闭集是上面五值，而**现况实现发的是自造名 `dial_failed`** ⇒ **尚未对齐**；\n")
	b.WriteString("    后果：`D1–D8` 里「四态可分」这条判据**今天判不了**（四态的名字还没落到闭集上）。\n")
	b.WriteString("  · `where`：契约闭集是 `core|agent`，现况发的是 `local` ⇒ 同样待对齐（同一批拍板项）。\n")
	b.WriteString("  · 对齐这两处 = 改命令面源码 + 与 `P-114`–`P-117` 同批拍 → 登记在《开工记录-批A-20260920.md》批 D 节。\n")
	return b.String()
}
