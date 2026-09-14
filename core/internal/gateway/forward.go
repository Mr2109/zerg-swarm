// package gateway - 转发逻辑：将请求转发到子端 :8100/infer。
//
// 转发规则：
//   - POST http://{host}:8100/infer
//   - body = 原请求体 + "_path" 字段（值为原始端点路径如 /v1/responses）
//   - 认证头透传（Authorization / X-Auth-Token / x-api-key）
//   - 响应透传（非流式完整返回 / 流式逐 chunk）
//
// v1 血泪教训：
//   - 大 body 支持：按 Content-Length 循环读，>64KB 不截断
//   - SSE 流式逐 chunk 转发，不能缓冲完整响应再发（v1 教训：大响应缓冲导致 Codex 超时）

package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/gateway/adapter"
)

// forwardToBackend 转发请求到子端 :8100/infer。
//
// 参数：
//   - route: 路由结果（host + port）
//   - originalPath: 原始端点路径（如 /v1/responses）
//   - body: 原始请求体
//   - headers: 需要透传的认证头
//   - required: 本次请求的必需能力（待修补 #38 ②）——本函数内的换机 failover 会重新选路，
//     required 必须一并透传给 pickFallbackRoute，保证换机后的目标引擎重新过能力门槛。
//     没有必需能力的调用点（压缩/摘要等纯文本内部请求）显式传 nil。
//
// 返回：
//   - 子端响应（直接透传，不做格式转换）
func (g *Gateway) forwardToBackend(
	ctx context.Context,
	route *RouteResult,
	originalPath string,
	body []byte,
	headers http.Header,
	required []string,
) (*http.Response, error) {
	// 构造转发 URL：直接用 pickRoute 算好的 URL（已含 IP + 端口）
	forwardURL := route.URL
	log.Printf("🔄 forwarding to agent: %s", forwardURL)

	// 添加 "_path" 字段到请求体
	// v1 Python agent 根据 _path 转发到后端对应端点
	var reqMap map[string]interface{}
	if err := json.Unmarshal(body, &reqMap); err != nil {
		return nil, fmt.Errorf("failed to parse request body: %w", err)
	}

	// 添加 _path 字段
	reqMap["_path"] = originalPath

	// 重新序列化为 JSON
	forwardBody, err := json.Marshal(reqMap)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize request body: %w", err)
	}

	// 创建转发请求（用独立超时 context，不跟客户端 context 走）
	// v1 教训：客户端断开/超时不应中断后端推理（DS4 冷加载 60-90s）
	// 治本（2026-08-12）：不移除 defer cancel——但 cancel 不能在返回时执行！
	// 根因：defer cancel() 在函数返回时取消 forwardCtx → resp.Body 的 context 被取消
	// → 网关 io.Copy 读响应中途 context canceled（EOF 根因——读到 ~3902 截断）
	// 修复：cancel 延迟到响应体读完才调用（用 Once + 响应体包装）
	forwardCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	forwardReq, err := http.NewRequestWithContext(forwardCtx, "POST", forwardURL, bytes.NewBuffer(forwardBody))
	if err != nil {
		// v2.5.6 修复（2026-08-29 q2 go vet）: 提前 return 必须 cancel——防 context 泄漏（10min 定时器挂着）
		cancel()
		return nil, fmt.Errorf("failed to create forward request: %w", err)
	}

	// 设置 Content-Type（保持原请求的 Content-Type）
	if contentType := headers.Get("Content-Type"); contentType != "" {
		forwardReq.Header.Set("Content-Type", contentType)
	} else {
		forwardReq.Header.Set("Content-Type", "application/json")
	}

	// 认证透传：统一转成子端认的 X-Auth-Token
	// 客户端可能用三种认证（X-Auth-Token / Bearer / x-api-key），子端只认 X-Auth-Token
	token := adapter.ExtractAuthToken(headers)
	if token != "" {
		forwardReq.Header.Set("X-Auth-Token", token)
		log.Printf("🔐 passing through auth header: X-Auth-Token")
	}

	// 透传其他重要头（可选）
	for _, header := range []string{"User-Agent", "Accept"} {
		if value := headers.Get(header); value != "" {
			forwardReq.Header.Set(header, value)
		}
	}

	// 发起转发请求（v2.5.4.10 模型超时覆盖——适配器声明 timeout_sec）
	log.Printf("📤 sending forward request to %s", forwardURL)
	var resp *http.Response
	// 模型级超时覆盖（适配器声明——Qwen3.8 120s/Nemotron 60s——覆盖全局 90s）
	// v2.5.6 故障自愈（Mr2109 2026-08-28）: 语义修正——适配器 timeout_sec 注释是"首 token 超时"
	// 之前实现覆盖整个请求 context → 长生成（13万token 思考模型）永远 120s 被杀 → 熔断风暴根因
	// 修正: timeout_sec 只作用于首字节等待（ResponseHeaderTimeout）——总超时放宽给长生成
	if override := g.getRequestTimeout(modelName(reqMap)); override > 0 {
		overrideClient := &http.Client{
			Timeout: 45 * time.Minute, // 长生成总超时（13万token思考模型 27B ~75tok/s ≈ 29min——留余量）
			Transport: &http.Transport{
				Proxy:                 nil,
				DisableKeepAlives:     true,
				ResponseHeaderTimeout: time.Duration(override) * time.Second, // 首 token 超时（适配器声明）
			},
		}
		resp, err = overrideClient.Do(forwardReq)
	} else {
		resp, err = g.client.Do(forwardReq)
	}
	if err != nil {
		// v2.5.6 修复（2026-08-29 q2 go vet）: 请求失败提前 return 必须 cancel——防 context 泄漏
		cancel()
		// v2.5.4.9 C failover：转发失败（超时/连接错误——卡死检测）→ 换机器重试 1 次
		// 场景: X3 单槽卡死——ResponseHeaderTimeout 60s 触发——换本机/其他候选
		// reason 传真实错误原文：failover 会涨失败/熔断计数，计数必须带原因（快照 last_error 可见）
		// required 透传（待修补 #38 ②）：换机是重新选路，新目标引擎必须重新过按引擎的能力门槛。
		if failoverRoute, ferr := g.pickFallbackRoute(route, modelName(reqMap), fmt.Sprintf("backend %s forward failed: %v", route.Host, err), required); ferr == nil {
			log.Printf("🔄 C failover: %s forward failed (%v) — switching to %s", route.Host, err, failoverRoute.Host)
			return g.forwardToBackend(ctx, failoverRoute, originalPath, body, headers, required)
		} else {
			// 换机被能力硬门槛拦下（待修补 #38 ②）：把门槛错误**原样上抛**，保住统一原因码
			// （capability_unavailable/400）。不这样处理，真实原因会被下面的 transport 错误
			// 盖成 502 upstream_fail——拦截是对的，但原因丢了就不是「同一套原因码」。
			var ge *CapabilityGateError
			if errors.As(ferr, &ge) {
				return nil, ferr
			}
		}
		// 诊断（2026-09-14）：主控自身进程拨号失败时，做三层对照以定位层级
		// 背景：同机其它进程（终端/launchd/同签名身份）都能拨通本地址，唯独主控不行。
		g.logDialDiagnostics(forwardURL, err)
		return nil, fmt.Errorf("forward request failed: %w", err)
	}

	// v2.5.4.9 reasoning 兜底：思考模型（Nemotron/Qwen3.8）content 空时拼 reasoning_content
	// CA 调研发现: reasoning_fallback 标记位无人消费——这里真实现
	// 读 body → content 空 → 用 reasoning_content 填充 → 重新构造 resp
	resp = g.applyReasoningFallback(resp)
	if resp == nil {
		// v2.5.6 修复（2026-08-29 q2 go vet）: 响应处理失败也要 cancel——防 context 泄漏
		cancel()
		return nil, fmt.Errorf("failed to handle forward response")
	}

	// 治本（2026-08-12）：cancel 绑定到响应体——读完才取消 context
	// （不能在函数返回时 defer cancel——resp.Body 的 context 会立即取消导致读取截断）
	resp.Body = &cancelOnCloseBody{ReadCloser: resp.Body, cancel: cancel}

	return resp, nil
}

// cancelOnCloseBody 包装响应体——关闭时取消 context（响应读完/出错才 cancel）。
type cancelOnCloseBody struct {
	io.ReadCloser
	cancel func()
	once   sync.Once
}

func (b *cancelOnCloseBody) Close() error {
	err := b.ReadCloser.Close()
	b.once.Do(b.cancel)
	return err
}
