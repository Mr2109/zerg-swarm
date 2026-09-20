// client.go —— 打主控 HTTP 的最小客户端（薄壳的第二层：解析 → 打 HTTP → 渲染）。
//
// 三条纪律：
//
//	① **端点真源不写死**：端口从 `core/internal/statepath` 取（`ZERG_PORT` → 8580），
//	   与主控/网关/子端共用同一份真源（§4.1 K9）。
//	② **令牌永不进 argv**（§4.1 K11 · §九 M2 `C2`）：只从环境变量或用户配置文件读；
//	   上下文文件里令牌一律**掩码**显示。
//	③ **fail-closed**（§九 M20 `O3`）：打不到就立即失败、不换端点、不读缓存当结果、不问任何问题；
//	   错误里点名「打不到的是谁 + 端点来自哪一级」。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

type client struct {
	base  string
	token string
	hc    *http.Client
}

// tokenSource —— 令牌取自哪一级（用于错误里点名「端点/凭据来自哪一级」，§九 M20 `O5`）。
type tokenSource string

const (
	tokenFromEnv    tokenSource = "env:ZERG_TOKEN"
	tokenFromFile   tokenSource = "file:~/.zerg/token"
	tokenFromNone   tokenSource = "缺（未认证）"
	baseFromBuiltin             = "builtin-local（statepath.CoreBaseURL）"
)

func newClient() *client {
	return &client{
		base:  statepath.CoreBaseURL(),
		token: resolveToken(),
		hc:    &http.Client{Timeout: 10 * time.Second},
	}
}

func resolveToken() string {
	if v := strings.TrimSpace(os.Getenv("ZERG_TOKEN")); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	b, err := os.ReadFile(filepath.Join(home, ".zerg", "token"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func (c *client) tokenOrigin() tokenSource {
	switch {
	case strings.TrimSpace(os.Getenv("ZERG_TOKEN")) != "":
		return tokenFromEnv
	case c.token != "":
		return tokenFromFile
	default:
		return tokenFromNone
	}
}

// getJSON 取一个端点并解成 out。错误按 M20 的形状打印：谁打不到 · 端点 · 来源级 · 只报一次。
func (c *client) getJSON(path string, out any) error {
	req, err := http.NewRequest(http.MethodGet, c.base+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Auth-Token", c.token) // 唯一真源 header（§九 M2 C10：不发明第四种）
	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("打不到主控 %s%s（endpoint 来源: %s）: %w", c.base, path, baseFromBuiltin, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return &authError{status: resp.StatusCode, token: c.tokenOrigin()}
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("主控 %s 返回 %d（endpoint 来源: %s）", path, resp.StatusCode, baseFromBuiltin)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("主控 %s 的响应不是合法 JSON: %w", path, err)
	}
	return nil
}

type authError struct {
	status int
	token  tokenSource
}

func (e *authError) Error() string {
	return fmt.Sprintf("未认证（HTTP %d）：令牌来自 %s —— 请先备好令牌（`ZERG_TOKEN` 环境变量或用户配置文件），令牌永不进 argv", e.status, e.token)
}
