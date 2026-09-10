// internal/gateway/orchestrator_exec.go —— 编排器 ModelExecutor 实现
// 用网关现有后端调用逻辑（调本机/远程模型端点）
package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/gateway/orchestrator"
)

// gatewayExecutor 编排器执行器——按模型直连推理端点（不经网关，避免流式 EOF）。
type gatewayExecutor struct {
	client *http.Client
	// 模型 → 端点映射（脑 ornith X3 隧道 / 手 nanbeige 本机）
	endpoints map[string]string
	authToken string // 备用认证（网关路径用）
}

// newGatewayExecutor 创建编排器执行器。
// 2026-08-12 改：所有模型走网关统一路由（8082）——不直连端点（虫族统一路由/负载均衡/认证/排除本机生效）
func newGatewayExecutor(authToken string) *gatewayExecutor {
	return &gatewayExecutor{
		client: &http.Client{Timeout: 10 * time.Minute},
		authToken: authToken,
		endpoints: map[string]string{
			// 全部走网关——网关按模型名路由（ornith→X3 / gemma→候选 / Qwable→候选）
			// 排除本机模式：网关路由自动跳过 local（参考模型候选 X3）
			"*": "http://127.0.0.1:8082/v1",
		},
	}
}

// Execute 执行模型调用，返回文本。
func (e *gatewayExecutor) Execute(ctx context.Context, model string, prompt string, maxTokens int) (string, error) {
	resp, err := e.ExecuteWithResponse(ctx, model, prompt, maxTokens)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}
func (e *gatewayExecutor) ExecuteWithResponse(ctx context.Context, model string, prompt string, maxTokens int) (*orchestrator.ModelResponse, error) {
	body := map[string]interface{}{
		"model":      model,
		"messages":   []map[string]string{{"role": "user", "content": prompt}},
		"max_tokens": maxTokens,
	}
	data, _ := json.Marshal(body)
	// 按模型选端点（未配置的模型走默认 8082 网关）
	endpoint := e.endpoints[model]
	if endpoint == "" {
		endpoint = "http://127.0.0.1:8082/v1"
	}
	endpoint += "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(data))
	if err != nil {
		return &orchestrator.ModelResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	// 网关内部请求带认证头（避免被鉴权拒绝）
	if e.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+e.authToken)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return &orchestrator.ModelResponse{}, fmt.Errorf("调用 %s 失败: %w", model, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		rb, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
		return &orchestrator.ModelResponse{}, fmt.Errorf("模型 %s 返回 %d: %s", model, resp.StatusCode, string(rb))
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content         string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return &orchestrator.ModelResponse{}, err
	}
	if len(out.Choices) == 0 {
		return &orchestrator.ModelResponse{}, fmt.Errorf("模型 %s 空响应", model)
	}
	// 思考模型（ornith）content 可能为空——输出在 reasoning_content
	content := out.Choices[0].Message.Content
	if content == "" {
		content = out.Choices[0].Message.ReasoningContent
	}
	return &orchestrator.ModelResponse{
		Content: content,
		Tokens:  out.Usage.TotalTokens,
	}, nil
}
