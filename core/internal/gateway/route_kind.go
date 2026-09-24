package gateway

import "strings"

// 机器种类（2026-09-25 Mr2109 定 · 设计稿 v1.7 §二）——溢出序：ai → mini → work。
//
//	`ai`   = AI 专用机（如 x3）：默认承接任务 · 粘性/择优真正起作用的档
//	`mini` = 小型机（类似 Mac mini 这种「内容小」的机器）：只接装得下的模型（装不下由能力硬门挡）
//	`work` = 工作机（Mr2109 的工作机，如 Mr2109）：**最后才用**
//
// 契约（最保守原则 ✓）：**未标 kind / 标了不认识的值 ⇒ 一律按 `work` 处理** ——
// 未声明种类的机器不得被当成 AI 专用机优先使用；宁可少用，不可误用他的工作机。
const (
	kindAI   = "ai"
	kindMini = "mini"
	kindWork = "work"
)

// normalizeKind 归一化种类名：大小写/空白容错；不认识 ⇒ work。
func normalizeKind(k string) string {
	switch strings.ToLower(strings.TrimSpace(k)) {
	case kindAI:
		return kindAI
	case kindMini:
		return kindMini
	default:
		return kindWork
	}
}

// kindRank 档序值（越小越优先）：ai=0 · mini=1 · work=2。
func kindRank(k string) int {
	switch normalizeKind(k) {
	case kindAI:
		return 0
	case kindMini:
		return 1
	default:
		return 2
	}
}

// machineKind 取某机器在机队配置里的种类（fleet.yaml `fleet:` 段 ⇒ FleetNode.Kind）。
func (g *Gateway) machineKind(host string) string {
	if g == nil || g.config == nil {
		return ""
	}
	if n, ok := g.config.Fleet[host]; ok {
		return n.Kind
	}
	return ""
}

// kindTier 返回某 host 的档序值（供候选循环比档序用）。
func (g *Gateway) kindTier(host string) int { return kindRank(g.machineKind(host)) }

// kindName 返回某 host 的规范化种类名（供日志用）。
func (g *Gateway) kindName(host string) string { return normalizeKind(g.machineKind(host)) }
