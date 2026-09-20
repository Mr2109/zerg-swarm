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
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

type client struct {
	base       string
	token      string
	stdinToken bool
	hc         *http.Client
}

// tokenSource —— 令牌取自哪一级（别名的目的：错误信息里回显同一处真源）。
// ★ 取值域的唯一真源在 `credentials.go`（`tokenSourceName` + `resolveCredentials`）——
// 本文件**不再自己读环境变量/文件**（`C1`：全命令面只有一个取令牌函数）。
type tokenSource = tokenSourceName

const (
	tokenFromEnv    = tokenSourceEnv
	tokenFromFile   = tokenSourceFile
	tokenFromNone   = tokenSourceNone
	baseFromBuiltin = "builtin-local（statepath.CoreBaseURL）"
)

// stdinTokenWanted / stdinTokenRead —— `--token-stdin` 的两个状态位（进程级；命令面无常驻状态，
// 一次调用里最多读一次 stdin —— 读两次会把管道里后面那点内容吃掉）。
var (
	stdinTokenWanted bool
	stdinTokenRead   bool
)

// newClient 造客户端：令牌一律经**唯一入口**（`C1`）取；给了 `--token-stdin` 则从 stdin 读
// （**不进 argv** · `C2`）——stdin 为空/读失败时**不静默退回文件**，而是按「缺令牌」报 4。
func newClient() *client {
	tok, _ := resolveCredentials()
	c := &client{base: statepath.CoreBaseURL(), token: tok, hc: &http.Client{Timeout: 10 * time.Second}}
	if stdinTokenWanted && !stdinTokenRead {
		stdinTokenRead = true
		c.stdinToken = true
		c.token = readTokenFromStdin(os.Stdin)
		if c.token == "" {
			c.token = ""
		}
	}
	return c
}

// resolveToken 保留这个小包装只为少改调用点：它**只**转发到唯一入口。
func resolveToken() string {
	v, _ := resolveCredentials()
	return v
}

func (c *client) tokenOrigin() tokenSource {
	if c.stdinToken {
		return tokenSourceStdin
	}
	if c.token == "" {
		return tokenFromNone
	}
	if _, src := resolveCredentials(); src != tokenSourceNone {
		return src
	}
	return tokenFromFile
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
		// §九 M7：网络层失败 = `unreachable`（**不是** `failed`）——
		// AI 自愈靠这个区分「去查链路」还是「去查代码」（§十二 P-013 ③）。
		return &netError{msg: fmt.Sprintf("打不到主控 %s%s（endpoint 来源: %s）", c.base, path, baseFromBuiltin), cause: err}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return &authError{status: resp.StatusCode, token: c.tokenOrigin()}
	}
	if resp.StatusCode == http.StatusConflict || resp.StatusCode == http.StatusTooManyRequests {
		return &httpError{path: path, status: resp.StatusCode, body: string(body)}
	}
	if resp.StatusCode != http.StatusOK {
		return &httpError{path: path, status: resp.StatusCode, body: string(body)}
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("主控 %s 的响应不是合法 JSON: %w", path, err)
	}
	return nil
}

// netError —— 链路层失败（kind=unreachable · 退码 12）。
type netError struct {
	msg   string
	cause error
}

func (e *netError) Error() string { return e.msg }
func (e *netError) Unwrap() error { return e.cause }

// httpError —— 主控回了非 2xx（不认证/冲突之外的那些）。
type httpError struct {
	path   string
	status int
	body   string
}

func (e *httpError) Error() string {
	return fmt.Sprintf("主控 %s 返回 %d（endpoint 来源: %s）", e.path, e.status, baseFromBuiltin)
}

type authError struct {
	status int
	token  tokenSource
}

func (e *authError) Error() string {
	return fmt.Sprintf("未认证（HTTP %d）：令牌来自 %s —— 请先备好令牌（`ZERG_TOKEN` 环境变量或用户配置文件），令牌永不进 argv", e.status, e.token)
}
