// Package ffp — 工具调用格式反馈协议(Format Feedback Protocol)
// 2026-09-08 实施,设计稿: 已归档至 Zerg-归档/v2.5.10/01-设计/设计-工具调用格式反馈协议.md
// 原则: 格式错误是"执行前的系统层异常"——反馈=教学(分类+原文回显+最小合法示例),
// 让模型看见自己发了什么、缺了什么、抄什么能过——而非干错误文本(实测空 command 死循环根因)。
// 独立 leaf 包: agent(exec.go)/loopcore(run.go)/chat(chat_tool_runtime.go) 三方引用,防 import 环。
package ffp

import (
	"fmt"
	"strings"
)

// Prefix — 系统断言首行。模型读到首行即知"这是格式错,不是逻辑/执行失败,勿原样重发"。
const Prefix = "⚠️ 工具调用格式错误(系统断言——非执行失败——请勿原样重发)"

// Is — 判定消息是否为 FFP 格式反馈(loop 层据此决定: 直通不加"执行失败"包装、不计 exec 失败)。
func Is(msg string) bool {
	return len(msg) >= len(Prefix) && msg[:len(Prefix)] == Prefix
}

// In — 容忍"工具名: "前缀的包含判定(executeTool 错误带 toolName 前缀——如 "bash: ⚠️…")
func In(msg string) bool {
	return strings.Contains(msg, Prefix)
}

// Echo — 原文回显窗口(头 headN + 尾 headN,中间省略标注——与 bash 溢出落盘同思想)。
// 让模型看见自己实际发出的调用原文;超长只露两端,防爆上下文。
func Echo(raw string, headN int) string {
	if raw == "" {
		return "<空/缺失>"
	}
	r := []rune(raw)
	if len(r) <= headN*2 {
		return raw
	}
	return string(r[:headN]) + fmt.Sprintf("[省略 %d 字符]", len(r)-headN*2) + string(r[len(r)-headN:])
}

// EchoSafe — Echo 的任意值版(模型参数值可能是数字/布尔/对象——统一格式化)
func EchoSafe(v any) string {
	return Echo(fmt.Sprint(v), 200)
}

// Build — 组装 FFP 教学文本。
//
//	kind:      确定性分类(如 "参数缺失: bash.command")
//	sent:      模型实际发出的片段原文(内部 Echo 窗口化)
//	expected:  parser 期望(点名缺什么)
//	example:   最小合法示例(1 行可抄 JSON)
//	guide:     下一步引导
func Build(kind, sent, expected, example, guide string) string {
	return fmt.Sprintf("%s\n\n[格式分类] %s\n[你上一条发送] %s\n[parser 期望] %s\n[最小合法示例] %s\n[guide] %s",
		Prefix, kind, Echo(sent, 200), expected, example, guide)
}
