package agent

// checker.go — 验证器实现（2026-08-13）
// 三种验证器：TestChecker（确定性命令）、LLMJudge（独立模型评审）、ComboChecker（组合）
// 依赖：标准库 exec/os/time/bytes/strings + net/http/encoding/json

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// LLMJudge 网关请求/响应结构

// LLMJudgeRequest — 发送给模型网关的请求体
type LLMJudgeRequest struct {
	Model    string       `json:"model"`
	Messages []LLMMessage `json:"messages"`
}

// LLMMessage — 单条消息（role + content）
type LLMMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// LLMJudgeResponse — 网关返回的响应体
type LLMJudgeResponse struct {
	Error   *LLMError   `json:"error,omitempty"`
	Choices []LLMChoice `json:"choices"`
}

// LLMError — 网关业务错误
type LLMError struct {
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

// LLMChoice — 单个 choice
type LLMChoice struct {
	Message      LLMMessage `json:"message"`
	FinishReason string     `json:"finish_reason,omitempty"`
}

// 1. TestChecker — 确定性验证（跑命令，exit 0 → 通过）

// TestChecker — 通过执行外部命令验证状态
// 用法：TestChecker{Command: "go test ./..."}
// Pass 判定：exit code == 0 → (true, 输出)；非 0 → (false, 输出)
type TestChecker struct {
	Command string // 要执行的命令（如 "go test ./..."）
}

// Pass — 执行命令并判定结果
//
// ctx     — 上下文（可取消/超时）；nil 时用 60s 默认超时
// contract — 契约字符串（描述"期望通过什么"，用于在失败时输出）
//
// 返回 (true, stdout+stderr) → 命令成功
// 返回 (false, stdout+stderr) → 命令失败（附契约描述）
func (tc TestChecker) Pass(ctx interface{}, contract string) (bool, string) {
	// 解析上下文
	c, cancel := defaultContext(ctx)
	defer cancel()

	// 构建命令：在 shell 中执行（支持管道、重定向等）
	cmd := exec.CommandContext(c, "sh", "-c", tc.Command)

	// 收集 stdout + stderr 到同一 buffer
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// 执行
	err := cmd.Run()
	out := stdout.String() + stderr.String()

	if err != nil {
		// 命令失败：附契约描述
		return false, fmt.Sprintf("%s\n契约: %s", out, contract)
	}
	return true, out
}

// 2. LLMJudge — 独立模型评审（防自评偏差）

// LLMJudge — 通过独立模型网关评审内容是否达标
// 用法：LLMJudge{GatewayURL: "...", AuthToken: "...", Model: "gpt-4o-mini"}
// 设计：独立模型评审，防止 Agent 自评偏差
type LLMJudge struct {
	GatewayURL string // 模型网关地址（如 "https://api.openai.com/v1/chat/completions"）
	AuthToken  string // API 密钥
	Model      string // 模型名（如 "gpt-4o-mini"）
}

// Pass — 调网关评审内容 vs 契约
//
// ctx     — 可选额外上下文；nil 或无法解析则不附加
// contract — 契约字符串
//
// 返回 (true, 评审意见) → 模型判定通过
// 返回 (false, 评审意见) → 模型判定不通过
func (lj LLMJudge) Pass(ctx interface{}, contract string) (bool, string) {
	// 配置检查
	if lj.GatewayURL == "" || lj.Model == "" {
		return false, "LLMJudge 配置不完整: GatewayURL 或 Model 为空"
	}

	// 构建评审 prompt：独立模型 + 客观评审
	judgePrompt := fmt.Sprintf("你是一位严格的验证评审员。请客观评审以下内容是否达标：\n\n契约: %s", contract)

	// 附加上下文（如果有）
	extraCtx := ""
	if ctx != nil {
		if s, ok := ctx.(string); ok && s != "" {
			extraCtx = "\n\n额外信息:\n" + s
		}
	}

	// 构造请求
	req := LLMJudgeRequest{
		Model: lj.Model,
		Messages: []LLMMessage{
			{Role: "system", Content: judgePrompt},
			{Role: "user", Content: extraCtx},
		},
	}

	// JSON 序列化
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return false, fmt.Sprintf("请求序列化失败: %v", err)
	}

	// HTTP 请求
	httpReq, err := http.NewRequest("POST", lj.GatewayURL, bytes.NewReader(reqBytes))
	if err != nil {
		return false, fmt.Sprintf("构造 HTTP 请求失败: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if lj.AuthToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+lj.AuthToken)
	}

	// 执行请求（30s 超时）
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return false, fmt.Sprintf("网关请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Sprintf("读取响应失败: %v", err)
	}

	// 状态检查
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Sprintf("网关返回 %d: %s", resp.StatusCode, string(respBody))
	}

	// 解析响应
	var llmResp LLMJudgeResponse
	if err := json.Unmarshal(respBody, &llmResp); err != nil {
		return false, fmt.Sprintf("响应解析失败: %v\n原始: %s", err, string(respBody))
	}

	// 错误处理
	if llmResp.Error != nil {
		return false, fmt.Sprintf("网关业务错误: %s", llmResp.Error.Message)
	}

	// 提取评审意见
	if len(llmResp.Choices) == 0 {
		return false, "网关返回空 choices"
	}

	judgment := strings.TrimSpace(llmResp.Choices[0].Message.Content)

	// 判定：输出包含"通过" → 通过，否则不通过
	if strings.Contains(judgment, "通过") && !strings.Contains(judgment, "不通过") {
		return true, judgment
	}
	return false, judgment
}

// 3. ComboChecker — 组合验证（全部通过才 done）

// ComboChecker — 依次执行多个验证器，全部通过才判定为 done
type ComboChecker struct {
	Checkers []Checker // 要依次执行的验证器列表
}

// Pass — 依次跑所有 checker，一个失败就终止
//
// ctx     — 传递给每个 checker 的上下文
// contract — 契约字符串（仅传递给第一个 checker，其余传空）
//
// 返回 (true, "所有 N 项验证通过") → 全过
// 返回 (false, "第 i 项失败: xxx")  → 某一项失败
func (cc ComboChecker) Pass(ctx interface{}, contract string) (bool, string) {
	if len(cc.Checkers) == 0 {
		return true, "无验证器，视为通过"
	}

	for i, checker := range cc.Checkers {
		// 仅第一个 checker 收到原始契约
		c := contract
		if i > 0 {
			c = ""
		}
		passed, review := checker.Pass(ctx, c)
		if !passed {
			return false, fmt.Sprintf("[ComboChecker] 第 %d/%d 项验证失败: %s", i+1, len(cc.Checkers), review)
		}
	}

	return true, fmt.Sprintf("所有 %d 项验证通过", len(cc.Checkers))
}

// 工具函数

// defaultContext — 将 interface{} 上下文转为 context.Context
//
// nil        → 60s 默认超时
// context.Context → 直接使用
// 其他类型    → 60s 默认超时
func defaultContext(ctx interface{}) (context.Context, context.CancelFunc) {
	if ctx == nil {
		return context.WithTimeout(context.Background(), 60*time.Second)
	}
	if c, ok := ctx.(context.Context); ok {
		return c, context.CancelFunc(func() {})
	}
	return context.WithTimeout(context.Background(), 60*time.Second)
}
