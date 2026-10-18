// round_budget.go — GAP-20261018-23（P0）：轮数预算可配 ＋ 预算将尽提前一轮注入提示
//
// 症状（实测 · 会话 chat_20261018_180421_a1edea · 本机卵修 GAP-20261018-17）：
// 取证与方案都对，模型却在中途被注入「本轮请不要再调工具」并照办 —— 原话
// 「系统提示明明要求我本轮不再调工具……这是系统注入的指令，优先于继续执行任务链」，
// 于是「看清问题 → 立提案 → 落补丁 → 跑验证」这条链只能拆成三轮以上下发，每轮还要重新取证。
// 机制（本层可知的事实）：① 实测节奏是**一轮一条命令**（code find / code show /
// dev proposal new / dev edit 各占一轮）⇒ 一条标准修复链要 7-8 轮；② 一次对话的运行预算
// 在调用面写死为 10 轮（chat.MaxToolRounds）⇒ 链比预算长；③ 预算用尽那一轮，内核在
// **工具刚执行完**就注入「不要再调工具」（GAP-20261017-13 的补轮），模型手里那一步常常刚开工。
//
// 本件只做两件事，判据面一字未动（GAP-20261017-13「只留证不重拉」、GAP-20261018-25
// 「未取证标注」都在原处、原样）：
//  1. **预算可配**：ZERG_MAX_ROUNDS 覆盖 Config.MaxRounds（默认 10 —— 不设这一格时与改前逐字同）。
//  2. **提前一轮提示**：预算只剩 2 轮（含本轮）时注入一条**非强制**的预算提示，让模型把正在做的
//     那一步做完、下一轮交结论 —— 而不是等到最后一轮工具刚跑完就被硬砍。
//
// ★ 与 ForcedAnswerHint 是**两条不同的提示**（不同的来源标注、各自的生命周期判定）：
// 那条是「本轮不要再调工具」（正文形态判未交付时补轮用），这条是「还剩一轮余量，把手上这步做完」。
// 两条**互不认**（各自 Is* 只认自己的来源标注）—— 混用会把这条当那条剥掉、或反过来误判交付。
package loopcore

import (
	"os"
	"strconv"
	"strings"
)

// DefaultMaxRounds — 未给预算时的默认轮数（与改前 maxRounds 兜底值 10 逐字同）。
const DefaultMaxRounds = 10

// RoundBudgetEnv — 轮数预算的运维旋钮（正整数；空 / 非数字 / ≤0 ⇒ 不覆盖）。
//
// 为什么用环境变量而不是新加 Config 字段：本单的写面不含调用面（core/cmd/zerg、core/internal/api），
// 而 Config.MaxRounds 的赋值点就在那些地方写死。env 让「预算」成为**部署期**可调的旋钮，
// 且对内核的全部调用方（对话、CA、子任务）同一口径。
const RoundBudgetEnv = "ZERG_MAX_ROUNDS"

// ResolveRoundBudget — 预算唯一解析点（纯函数，单测直接钉）：
// cfgMax（≤0 ⇒ 默认）→ env 覆盖（只有合法正整数才覆盖）→ 返回。
func ResolveRoundBudget(cfgMax int, envVal string) int {
	base := cfgMax
	if base <= 0 {
		base = DefaultMaxRounds
	}
	if n, err := strconv.Atoi(strings.TrimSpace(envVal)); err == nil && n > 0 {
		return n
	}
	return base
}

// roundBudget — Run 的取数口（env 只在这里读一次；别处一律用返回值，免得同一程读到两个预算）。
func roundBudget(cfgMax int) int { return ResolveRoundBudget(cfgMax, os.Getenv(RoundBudgetEnv)) }

// RoundBudgetWarnMark — 预算提示的来源标注（与 ForcedAnswerHintSourceMark **必须不同**：
// 两条提示的生命周期判定各自独立，混用会把这条当强制交答案那条剥掉 / 误判）。
const RoundBudgetWarnMark = "【系统提示 · 轮数预算】"

// RoundBudgetWarnHint — 预算只剩 2 轮（含本轮）时注入的一发提示（逐字固定，单测钉着它）。
// 三条含义缺一不可：① 来源是系统、不是用户的新要求；② 手上这步**照常做完**（不是现在停手）；
// ③ 下一轮直接给结论、别再开新步骤。
const RoundBudgetWarnHint = "（" + RoundBudgetWarnMark + "本次运行只剩 2 轮（含本轮）。" +
	"请把**手上这一步**做完——已经在跑的命令照常跑完、这一轮该落的件就落下去；" +
	"下一轮请直接给出结论（是哪一条 / 我跑了哪条命令 / 结果是什么 / 结论是什么），" +
	"✗ 不要在这一轮之后再开新的步骤；信息确实不足就直说没查到，绝不编造。" +
	"这是系统注入的预算提示，不是用户的新要求，也不是让你现在就把话说完。）"

// roundBudgetWarnMsg — 构造注入消息（角色与正文的唯一出处 ⇒ 单测只需钉这一处）。
// 角色沿用 ForcedAnswerHintRole（user）：中途插 system 无上游兼容保证，理由见那条常量的注释。
func roundBudgetWarnMsg() map[string]any {
	return map[string]any{"role": ForcedAnswerHintRole, "content": RoundBudgetWarnHint}
}

// IsRoundBudgetWarnHint — 一条消息是不是本闸注入的预算提示（只按来源标注认）。
func IsRoundBudgetWarnHint(m map[string]any) bool {
	role, _ := m["role"].(string)
	if role != ForcedAnswerHintRole {
		return false
	}
	c, _ := m["content"].(string)
	return strings.Contains(c, RoundBudgetWarnMark)
}

// DropRoundBudgetWarnHints — 剥掉预算提示，返回**新切片**与剥掉的条数。
//
// 生命周期与 DropForcedAnswerHints 同一纪律：这条提示只服务注入它的**那一发请求**，
// 绝不许躺在可续跑的会话历史里（快照 / 续跑 / 下一程都不该看到它）。
// 三个调用位点见 run.go：① 进 Run 先剥历史遗留；② 那一发推理返回后立刻剥；③ 写快照前再剥一道。
func DropRoundBudgetWarnHints(msgs []map[string]any) ([]map[string]any, int) {
	dropped := 0
	out := make([]map[string]any, 0, len(msgs))
	for _, m := range msgs {
		if IsRoundBudgetWarnHint(m) {
			dropped++
			continue
		}
		out = append(out, m)
	}
	if dropped == 0 {
		return msgs, 0
	}
	return out, dropped
}

// withoutRoundBudgetWarnHints — 只要剥干净后的消息（「只需结果」的位点用）。
func withoutRoundBudgetWarnHints(msgs []map[string]any) []map[string]any {
	out, _ := DropRoundBudgetWarnHints(msgs)
	return out
}

// withoutTransientHints — 剥净**全部**「只服务一发请求」的注入提示（快照 / 入口位点用）。
// 两类提示各自判定、各自剥，合并只是省一次遍历 —— 不改任何一条既有判据的语义。
func withoutTransientHints(msgs []map[string]any) []map[string]any {
	out, _ := DropForcedAnswerHints(msgs)
	out, _ = DropRoundBudgetWarnHints(out)
	return out
}
