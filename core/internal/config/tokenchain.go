// tokenchain.go —— 共享令牌的**唯一解析链**（GAP-20261017-02 / GAP-20261017-04 收敛）。
//
// 一条链，三处读取面同源（CLI · 主控 · 子端）：
//
// TOKEN-CHAIN: ZERG_TOKEN > ~/.zerg/token > ZERG_AUTH_TOKEN > ZERG_API_TOKEN
//
// 即：`env:ZERG_TOKEN` > `file:~/.zerg/token` > `env:ZERG_AUTH_TOKEN`（旧名·兼容）> `env:ZERG_API_TOKEN`（旧名·兼容）。
//
// ★ 旧名只**降级**、不删：它们排在文件之后，命中也只是「最后一级」。
// ★ 五不进照旧：令牌**不进 argv / URL / 日志 / 聊天 / 入库**。本件只产**来源标记**，
// 任何报话都不得拼接令牌值（`zerg context ls` 的 `token_source` 列即来源标记）。
// ★ 子端在另一 Go module（`agent/`）⇒ 不能 import 本件；那边必须逐字同序，
// 对拍判据见 `tokenchain_parity_test.go`（三处源文件逐字比）。
package config

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

// 令牌的三个环境变量名（真源；✗ 别在别处再抄一份字面量）。
const (
	TokenEnvPrimary      = "ZERG_TOKEN"
	TokenLegacyAuthToken = "ZERG_AUTH_TOKEN"
	TokenLegacyAPIToken  = "ZERG_API_TOKEN"
)

// TokenSource —— 令牌取自哪一路（**只有来源，永不含令牌值**）。
type TokenSource string

const (
	TokenSourceEnv        TokenSource = "env:ZERG_TOKEN"
	TokenSourceFile       TokenSource = "file:~/.zerg/token"
	TokenSourceLegacyAuth TokenSource = "env:ZERG_AUTH_TOKEN(旧名·兼容)"
	TokenSourceLegacyAPI  TokenSource = "env:ZERG_API_TOKEN(旧名·兼容)"
	// TokenSourceConfig —— 兜底：fleet.yaml 的 `auth.token`（库内零明文，只作最后兼容）。
	TokenSourceConfig TokenSource = "config:fleet.yaml#auth.token"
	// TokenSourceMissing —— 三路都空（调用方必须显式处理，不得静默放行）。
	TokenSourceMissing TokenSource = "缺（未配置令牌）"
)

// TokenChain —— **优先级链唯一字面量**（`zerg help credentials` 与来源报话共用）。
// 对拍判据按这一串逐字核三处读取面。
const TokenChain = string(TokenSourceEnv) + " > " + string(TokenSourceFile) + " > " +
	string(TokenSourceLegacyAuth) + " > " + string(TokenSourceLegacyAPI)

// IsLegacy —— 这一级是不是兼容旧名（降级档）。
func (s TokenSource) IsLegacy() bool {
	return s == TokenSourceLegacyAuth || s == TokenSourceLegacyAPI
}

var (
	// tokenLegacyOnce 用指针持有，测试可整体换一枚新的（avoid copylocks）。
	tokenLegacyOnce = &sync.Once{}
	// tokenLegacySink 只在测试里换（生产恒 os.Stderr）；换它不改变「一次」的语义。
	tokenLegacySink io.Writer = os.Stderr
)

// AnnounceLegacyTokenSource —— 旧名命中时**出声一次**（进程级），只说「从哪一路取到」。
// ✗ 绝不回显令牌：参数是来源标记，本函数拿不到令牌值（签名即不变量）。
func AnnounceLegacyTokenSource(src TokenSource) {
	if !src.IsLegacy() {
		return
	}
	tokenLegacyOnce.Do(func() {
		fmt.Fprintf(tokenLegacySink, "⚠ 令牌取自兼容旧名 %s —— 旧名已降级为兼容别名（保留不删）；"+
			"建议改用 %s 或 %s（令牌值不进日志）\n", src, TokenSourceEnv, TokenSourceFile)
	})
}

// ResolveAuthTokenWithSource —— **唯一解析逻辑**：返回令牌 + 来源标记。
// 纯读取：不打印、不缓存、不落盘、不写日志（报话由调用方按 `TokenSource` 出）。
func ResolveAuthTokenWithSource() (string, TokenSource) {
	if v := strings.TrimSpace(os.Getenv(TokenEnvPrimary)); v != "" {
		return v, TokenSourceEnv
	}
	if p := TokenFilePath(); p != "" {
		if b, err := os.ReadFile(p); err == nil {
			if v := strings.TrimSpace(string(b)); v != "" {
				return v, TokenSourceFile
			}
		}
	}
	if v := strings.TrimSpace(os.Getenv(TokenLegacyAuthToken)); v != "" {
		return v, TokenSourceLegacyAuth
	}
	if v := strings.TrimSpace(os.Getenv(TokenLegacyAPIToken)); v != "" {
		return v, TokenSourceLegacyAPI
	}
	return "", TokenSourceMissing
}
