// topics.go —— 帮助主题的**唯一分派面**（`zerg help <主题>`）。
//
// 为什么把它从 `main.go` 的 switch 里搬出来：主题会一直加（批 B 一次加了 7 个），
// 每次加一个都要改主流程 ⇒ 两处写法迟早漂。这里用**一张表**当分派真源：
// 表里没有的主题 ⇒ 报「未知帮助主题」+ 列可用主题（表就是列清单的真源）。
package main

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// helpTopic —— 一个帮助主题（名字 → 渲染函数）。
type helpTopic struct {
	Name  string
	Brief string
	Text  func() string
}

// helpTopics —— 主题表（**唯一真源**：分派、错误信息里的「可用主题」都从这里来）。
func helpTopics() []helpTopic {
	return []helpTopic{
		{"exit-codes", "退码表（唯一真源）", helpExitCodes},
		{"config", "五级配置优先级 + 凭据四条", helpConfig},
		{"dangerous", "危险动作清单（23 条 · 三态）", helpDangerous},
		{"contract", "输出契约：三层版本 + 包封 + 字段稳定性承诺", helpContract},
		{"errors", "error.kind 闭集 + 可重试性 + 自愈入口", helpErrors},
		{"idempotency", "幂等四字段（幂等档/重跑语义/生效语义/危险档）", helpIdempotency},
		{"locks", "并发与锁：三级粒度 + 三时钟 + slot 版本号", helpLocks},
		{"long-tasks", "长任务：三档旗标 + 默认档 + 句柄", helpLongTasks},
		{"remote", "远端语义：--context 与 --node 两轴 + 名册真源", helpRemote},
		{"version", "版本协商：三层版本 + 兼容窗口（三类分开）", helpVersionTopic},
		{"watch", "事件 / 订阅通道：单端点 + Accept 协商 + 保留窗口", helpWatch},
		{"credentials", "凭据：优先级链一条 + 令牌五不进 + 401/403 归一", helpCredentials},
		{"wall", "茧壁边界：layer 三档 + 分区表 + argv 硬判据", helpWall},
		{"human-machine", "人机双模：三态 + 两层承诺 + 分页/颜色/format 四条", helpHumanMachine},
		{"registry", "契约登记表：真源口径 + 六处先例 + 变更流程 V0–V7", helpRegistry},
		{"offline", "离线/降级：单端点 + 四条「不偷偷」+ 例外清单 F-1–F-4", helpOffline},
		{"plugins", "插件机制：`zerg-<名>` 约定 + 零注册表 + 信任声明（P14）", helpPlugins},
	}
}

// topicNames 排好序的主题名（错误信息与自描述用）。
func topicNames() []string {
	names := make([]string, 0, len(helpTopics()))
	for _, t := range helpTopics() {
		names = append(names, t.Name)
	}
	sort.Strings(names)
	return names
}

// renderHelpTopic 分派一个主题；找不到 ⇒ 退码 2 + 列全部可用主题（K14 第三件）。
func renderHelpTopic(name string, stdout, stderr io.Writer) int {
	for _, t := range helpTopics() {
		if t.Name == name {
			fmt.Fprint(stdout, t.Text())
			return exitOK
		}
	}
	fmt.Fprintf(stderr, "%s: 未知帮助主题 %q\n", progName, name)
	fmt.Fprintf(stderr, "可用主题: %s\n", strings.Join(topicNames(), " · "))
	fmt.Fprintf(stderr, "See '%s --help'。\n", progName)
	return exitUsage
}
