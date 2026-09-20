// obs_bridge.go —— 审计与脱敏的**消费面**（§九 M9 / §十五.5 · §二十一 第 10 条）。
//
// 为什么要有这个文件（不是「为了凑两个 import」）：
//
//	已红第 10 条逐字：`internal/audit` 与 `obs/redact` 的 import 消费者 = **0** ——
//	**规则写了、没人接电**（fail-open 的同族病）。两条规矩在仓里都有实现、也都有文档，
//	但没有任何调用方 ⇒ 「脱敏」与「留痕」在真链路上**从来没发生过**。
//
// 本桥接把两件事接进**主控的对外面上**（都只加字段/只做变换，不改既有语义）：
//
//	① **脱敏**（§九 M2 `R` 面 · 调研-M2 §4.3）：「先脱敏后编码 + 失败即拒」——
//	   `writeError` / `writeErrorCode` 的 message 在**编码之前**过 `redact.RedactValue`。
//	② **效果指纹**（§九 M7 `E4`「不吞根因」· §十.9 `RC10`「写动作留痕」）：
//	   用 `audit.ArgsFingerprint` 给「同一次控制面请求」算一个**稳定摘要**，随响应回给调用方
//	   ⇒ 日志里那一行与响应里那一枚能对上（不靠人眼比对时间戳）。
//
// 边界（如实说）：本文件**不**引入 audit.Store（那是写盘面，属 §九 M19 `RC10` 的落点），
// 只用它的**指纹算法**（纯函数）—— 因此这里零副作用、可随时复用。
package api

import (
	"github.com/Mr2109/zerg-swarm/core/internal/audit"
	"github.com/Mr2109/zerg-swarm/core/internal/obs/redact"
)

// redactedMessage —— 先脱敏、再编码（§九 M2：脱敏在**编码之前**）。
// 失败即拒的口径：`RedactValue` 是纯函数，不会失败；若将来换成「会拒」的实现，
// 拒的时候**不许**退回原串（那等于脱敏失效）—— 这条写在接缝上，防以后改坏。
func redactedMessage(msg string) string {
	return redact.RedactValue(msg)
}

// controlFingerprint —— 控制面请求的效果指纹（同参 ⇒ 同值；用于「日志 ↔ 响应」对账）。
// 用 `audit` 的算法而不是再写一份哈希：**一处算法**（§九 M5：一个对象一处算法）。
func controlFingerprint(op string, args map[string]any) string {
	fp, err := audit.ArgsFingerprint(args)
	if err != nil || fp == "" {
		return ""
	}
	return op + "#" + fp
}
