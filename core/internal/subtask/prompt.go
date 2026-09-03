package subtask

// prompt.go — 拆解轮/结晶轮 prompt 模板（设计 3.1/3.2 + G2 过程教训 + R9 错误清单反馈）

import (
	"fmt"
	"strings"
)

// DecomposePrompt — 拆解轮 prompt（强模型一次拆全——输出 JSON）
// replanFeedback 非空 = Replan 模式（带上已完成结晶+卡点——只拆剩余）
// lastStats 非空 = 重拆反馈（G3——上次拆解统计）
func DecomposePrompt(taskDesc string, replanFeedback string, lastStats string) string {
	var b strings.Builder
	b.WriteString("你是任务架构师。将以下任务分解为有序子任务（A→B→C→N）。\n\n")
	b.WriteString("【要求】\n")
	b.WriteString("1. 每个子任务 ≤1 个可验收目标（能写进 must_write_files/must_pass_cmds）\n")
	b.WriteString("2. 子任务间依赖显式声明（depends 数组——B 需要哪些前置步骤的产物）\n")
	b.WriteString("3. 每个子任务附带验收契约 contract（must_write_files=必须落盘的文件 / must_pass_cmds=必须通过的命令）\n")
	b.WriteString("4. 子任务数 2-8 个（太少=没拆——太多=膨胀）\n")
	b.WriteString("5. 每个子任务描述含：做什么/产出什么文件/完成判据\n")
	b.WriteString("6. 关键步骤（失败=整个任务无意义）标 critical:true\n")
	b.WriteString("7. 步骤目标必须来自任务原文——不要发明任务没要求的东西\n")
	b.WriteString("8. 若任务要求写报告：报告步骤的 goal 必须注明「报告内容≥100字——记录: 做了什么/结果/关键数据/验证证据」——禁止一句话空报告\n")
	b.WriteString("9. 【输出完整性】JSON 必须完整闭合（steps 数组+根对象）——先在内部规划好全部步骤再一次性输出——禁止中途截断\n\n")
	b.WriteString("【任务原文】\n" + taskDesc + "\n\n")
	if replanFeedback != "" {
		b.WriteString("【重拆背景】以下为已完成阶段的结晶与当前卡点——只拆剩余部分，已完成阶段不要重复拆：\n" + replanFeedback + "\n\n")
	}
	if lastStats != "" {
		b.WriteString("【上次拆解统计（供参考——避免重蹈覆辙）】\n" + lastStats + "\n\n")
	}
	b.WriteString(`只输出 JSON（不要 markdown 标记不要解释）：
{"steps":[{"id":"A","goal":"...","produces":["文件"],"depends":[],"critical":false,"contract":{"must_write_files":[],"must_pass_cmds":[]}}],"rationale":"拆解思路一句话"}`)
	return b.String()
}

// CrystallizePrompt — 结晶轮 prompt（模型提取关键数据+教训——程序已填骨架其余字段）
func CrystallizePrompt(step Step, exitKind string, contractResults []string, recentActivity string) string {
	var b strings.Builder
	b.WriteString("你是阶段总结员。以下阶段刚执行完毕——从执行记录中提取关键信息供后续阶段使用。\n\n")
	b.WriteString(fmt.Sprintf("【阶段目标】%s\n", step.Goal))
	b.WriteString(fmt.Sprintf("【执行结果】%s\n", exitKind))
	for _, r := range contractResults {
		b.WriteString("  · " + r + "\n")
	}
	b.WriteString("\n【执行记录（截选）】\n" + truncateRunes(recentActivity, 2500) + "\n\n")
	b.WriteString(`只输出 JSON：
{"key_data":["精确值——路径/命令/数字/错误原文——至少1条"],"lessons":["过程教训——坑与绕法——0-2条"],"influence":"对后续阶段的影响——1-3句"}`)
	return b.String()
}

// AdaptPlanPrompt — 计划缓存适配 prompt（R8/APC——3.8 节）
func AdaptPlanPrompt(template string, newTaskDesc string) string {
	return fmt.Sprintf(`以下是历史任务的拆解计划模板。请将其适配到新任务（保持骨架——只改参数/路径/具体内容）。

【历史计划模板】
%s

【新任务】
%s

只输出适配后的 JSON（格式同模板）。`, template, newTaskDesc)
}
