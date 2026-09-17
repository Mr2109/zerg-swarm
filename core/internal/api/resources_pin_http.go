package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/config"
	"github.com/Mr2109/zerg-swarm/core/internal/tracectx"
)

// resources_pin_http.go —— pin/unpin 的默认动作侧：把主控的锁定意图转发到子端 agent 的 HTTP 接口
// （POST /pin、POST /unpin —— 批 3 的子端 Manager.Pin/Unpin 的动作路径，批 4 补上这根线）。
//
// 边界：只做转发与错误如实上报，不做任何进程动作；地址来自 fleet 配置（cfg.Fleet[host]）。
// 本机（local）没有独立子端 agent → **明确报错**，不假装成功（本机驻留由内置 LocalBack 管理）。

// SubEndPinController 是 ResourcePinController 的 HTTP 实现（转发到子端 agent）。
type SubEndPinController struct {
	// Token 是共享令牌（放到 X-Auth-Token；空则不带头）。
	Token string
	// Fleet 是机器名 → 子端地址（来自 fleet 配置）。
	Fleet map[string]config.FleetNode
	// Client 可注入（测试用）；nil 时用内置直连客户端。
	Client *http.Client
	// BaseURL 覆盖地址解析（测试注入假子端用）；nil 时按 Fleet 的 host:port。
	BaseURL func(host string) (string, bool)
}

// NewSubEndPinController 按 fleet 配置构造默认动作侧。
func NewSubEndPinController(cfg *config.FleetConfig, token string) *SubEndPinController {
	c := &SubEndPinController{Token: token}
	if cfg != nil {
		c.Fleet = cfg.Fleet
	}
	return c
}

// baseURL 解析子端基地址；ok=false 表示该机器没有可用的子端地址。
func (c *SubEndPinController) baseURL(host string) (string, bool) {
	if c.BaseURL != nil {
		return c.BaseURL(host)
	}
	n, ok := c.Fleet[host]
	if !ok || strings.TrimSpace(n.Host) == "" || n.Port <= 0 {
		return "", false
	}
	return fmt.Sprintf("http://%s:%d", n.Host, n.Port), true
}

// client 返回内置直连客户端（内部转发永远直连：禁用代理——Go 默认 Transport 会读系统代理，
// 把局域网 10.0.x 请求发给代理；并禁 keep-alive，避免子端空闲关连接导致复用断连）。
func (c *SubEndPinController) httpClient() *http.Client {
	if c.Client != nil {
		return c.Client
	}
	return &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			Proxy:             nil,
			DisableKeepAlives: true,
		},
	}
}

// PinResource 转发 pin 到子端。
func (c *SubEndPinController) PinResource(host, model string, ttlS int) error {
	return c.post(host, "/pin", map[string]interface{}{"model": model, "ttl_s": ttlS})
}

// UnpinResource 转发 unpin 到子端。
func (c *SubEndPinController) UnpinResource(host, model string) error {
	return c.post(host, "/unpin", map[string]interface{}{"model": model})
}

// post 发一次 JSON POST 到子端；非 200 或网络错误一律如实返回错误（不吞、不假装成功）。
func (c *SubEndPinController) post(host, path string, body map[string]interface{}) error {
	base, ok := c.baseURL(host)
	if !ok {
		return fmt.Errorf("机器 %s 无子端地址（本机无独立子端或未在 fleet 配置中）", host)
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, base+path, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("X-Auth-Token", c.Token)
	}
	// T1.6 传播（出站到子端）：/pin、/unpin 也是"主控→子端"的 HTTP 调用，带 traceparent
	// （无会话上下文 ⇒ 本侧新生成 root，如实反映"这是一次独立的资源操作"）。
	tracectx.Propagate(req.Header, nil, "", tracectx.ReplayMarked())
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("子端返回 %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	return nil
}
