// Package api - v2.5.6 程序定量驱动 Agent: 受控循环（执行轮核心）
// 每个小任务 = 受控循环——模型调工具直到完成——边界程序管
// 2026-08-25 Mr2109（D 定稿）——设计 docs/01-设计/设计-程序定量驱动Agent-20260824.md 16 章
package api

import (
	"fmt"
	"os/exec"
	"time"
)

// ControlledLoop 受控循环配置（边界参数——程序定——确定性）
type ControlledLoop struct {
	MaxRounds    int           // 循环上限（默认 5——一个小任务的合理轮数）
	VerifyCmd    []string      // 验证命令（程序检查证据——如 go test）
	Workdir      string        // 工作区（任务 git）
	FailFeedback string        // 失败反馈（重做时带——避免重复犯错）
	Timeout      time.Duration // 每轮超时（默认 5 分钟）
}

// NewControlledLoop 默认配置
func NewControlledLoop(workdir string) *ControlledLoop {
	return &ControlledLoop{
		MaxRounds: 5,
		Workdir:   workdir,
		Timeout:   5 * time.Minute,
	}
}

// WithVerifyCmd 设置验证命令（按任务类型——代码=go test/文档=文件检查）
func (c *ControlledLoop) WithVerifyCmd(cmd ...string) *ControlledLoop {
	c.VerifyCmd = cmd
	return c
}

// LoopResult 循环结果
type LoopResult struct {
	Success     bool   // 完成（验证过）
	RoundsUsed  int    // 用了多少轮
	FailReason  string // 失败原因（超限/验证不过）
	LastOutput  string // 最后一轮输出
	NeedRebuild bool   // 需要 git 回溯重做
}

// Verify 验证（程序检查证据——非模型自报）
// 执行验证命令——输出成功（exit 0）= 通过
func (c *ControlledLoop) Verify() (bool, string, error) {
	if len(c.VerifyCmd) == 0 {
		// v2.5.6 放宽: 无验证命令——模型响应即过（不要求工作区变化）
		// 原因: 有的步骤是"验证/检查"类——无新文件产出——"工作区有变化"误伤
		// 防假完成由 CommitRoundWithFS 的"空提交拦截"兜底（真没产出——git 空提交被拦——任务失败）
		return true, "模型已响应（无验证命令——提交层防假完成兜底）", nil
	}
	// 有验证命令——执行（如 go test）
	cmd := exec.Command(c.VerifyCmd[0], c.VerifyCmd[1:]...)
	cmd.Dir = c.Workdir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Sprintf("验证失败: %v\n%s", err, truncateStr(string(out), 300)), nil
	}
	return true, fmt.Sprintf("验证通过: %s", truncateStr(string(out), 100)), nil
}

// RunStep 受控循环（一个小任务）
// 伪代码:
//   for round in 1..MaxRounds:
//     调模型（当前小任务 + 上步总结 + 失败反馈）→ 模型调工具完成
//     验证（程序检查）→ 过 = 成功退出
//     不过 → 反馈重做（带失败信息）
//  超限 → NeedRebuild（git 回溯重做）
//
// 说明: 实际模型调用由调用方（执行器）做——本结构体提供边界控制与验证
//（模型调用走网关——见 zerg_executor.go——后续实施）
func (c *ControlledLoop) RunStep(step string, callModel func(step, feedback string) (string, error)) (*LoopResult, error) {
	result := &LoopResult{}
	feedback := c.FailFeedback // 初始失败反馈（可能是重做带入的）

	for round := 1; round <= c.MaxRounds; round++ {
		result.RoundsUsed = round
		// 调模型（当前小任务 + 失败反馈——如果有）
		output, err := callModel(step, feedback)
		if err != nil {
			feedback = fmt.Sprintf("模型调用失败: %v——请重试", err)
			continue
		}
		result.LastOutput = output

		// 验证（程序检查证据）
		ok, evidence, vErr := c.Verify()
		if vErr != nil {
			feedback = fmt.Sprintf("验证异常: %v——请重试", vErr)
			continue
		}
		if ok {
			result.Success = true
			return result, nil
		}
		// 验证不过——反馈重做（带失败信息——避免重复犯错）
		feedback = fmt.Sprintf("【失败反馈】%s——请修正后重做（不要重复同样的错误）", evidence)
		result.FailReason = evidence
	}
	// 超限——需要 git 回溯重做
	result.NeedRebuild = true
	result.FailReason = fmt.Sprintf("循环 %d 轮未通过验证——需要 git 回溯重做（失败: %s）", c.MaxRounds, result.FailReason)
	return result, nil
}

// truncateStr 截断字符串（超长——保留头尾）
func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n/2] + "..." + s[len(s)-n/2:]
}
