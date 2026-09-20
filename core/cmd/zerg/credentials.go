// credentials.go —— 统一凭据来源（§九 M2 · 调研-M2 §4.1 `C1`–`C12`）。
//
// **一条链，唯一一个函数**（`C1`）：`env > file`（§十二 `P-006` 定案取**甲**）。
// 全命令面只有这一处读令牌（`client.go` 也走它）—— 三处各读一遍 = 三套优先级 = 事故温床。
//
// 五不进（`C2`/`C3`/`C4` · `K11`）：令牌**不进 `argv`** · **不进 URL** · **不入库** ·
// **不进日志** · **不进聊天**。非交互场景用 `--token-stdin` 从标准输入读（不占命令行）。
//
// 三级后端（`M2` 的第三级）：三级后端**只从请求头收令牌**（`C10`：不发明第四种 header）。
// 缺令牌 ⇒ `exit 4` + `kind=unauthenticated`（`C7`）；`403` 与 `4` **并码、kind 分家**
// （§十二 `P-013` ④ → 本件落 `unauthenticated` / `forbidden` 两个 kind）。
package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

// tokenSourceName —— 令牌取自哪一级（错误信息里点名「凭据来自哪一级」· §九 M20 `O5`）。
type tokenSourceName string

const (
	tokenSourceEnv   tokenSourceName = "env:ZERG_TOKEN"
	tokenSourceFile  tokenSourceName = "file:~/.zerg/token"
	tokenSourceStdin tokenSourceName = "stdin:--token-stdin"
	tokenSourceNone  tokenSourceName = "缺（未认证）"
)

// credentialChain —— **优先级链唯一真源**（§十二 `P-006`：env > file）。
// 打印出来就是契约 §八 那一行的一部分（`zerg help credentials` 与 `zerg help config` 共用）。
const credentialChain = "env:ZERG_TOKEN > file:~/.zerg/token（非交互另可 --token-stdin；**旗标不从命令行传值**）"

// resolveCredentials 取令牌 + 它来自哪一级。**全命令面唯一入口**（`C1`）。
func resolveCredentials() (string, tokenSourceName) {
	if v := strings.TrimSpace(os.Getenv("ZERG_TOKEN")); v != "" {
		return v, tokenSourceEnv
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", tokenSourceNone
	}
	b, err := os.ReadFile(filepath.Join(home, ".zerg", "token"))
	if err != nil {
		return "", tokenSourceNone
	}
	v := strings.TrimSpace(string(b))
	if v == "" {
		return "", tokenSourceNone
	}
	return v, tokenSourceFile
}

// readTokenFromStdin —— `--token-stdin`（`C2`：非交互且**不把令牌写进命令行**）。
// 只读一行；读完即用即弃（不落盘、不回显、不进日志）。
func readTokenFromStdin(r io.Reader) string {
	b, err := io.ReadAll(io.LimitReader(r, 4096))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// helpCredentials —— `zerg help credentials`（§九 M2 的自描述面）。
func helpCredentials() string {
	var b strings.Builder
	b.WriteString("凭据与鉴权（§九 M2 · 调研-M2 §4.1 `C1`–`C12`）\n\n")
	b.WriteString("优先级链（**一条** · §十二 `P-006` 取甲）：\n  " + credentialChain + "\n\n")
	b.WriteString("令牌五不进（`C2`/`C3`/`C4` · 契约 §八）：\n")
	b.WriteString("  · **永不进 `argv`** —— 进程列表里看得到命令行；要非交互就给 `--token-stdin`（从 stdin 读）。\n")
	b.WriteString("  · **永不进 URL**（2026-09-15 已拍板）；\n")
	b.WriteString("  · **永不入库**（仓内零明文令牌；`.env` 不许被追踪）；\n")
	b.WriteString("  · **永不进日志** / **永不进聊天**（贴日志=泄露）。\n\n")
	b.WriteString("三级后端与 header（`C10`）：三级后端**只从请求头收令牌**；唯一真源 header = `X-Auth-Token`，\n")
	b.WriteString("**不许发明第四种**。网关/主控/子端的 header 投影表是同一份（一处真源）。\n\n")
	b.WriteString("缺令牌与服务端行为：\n")
	b.WriteString("  · 缺令牌 ⇒ `exit 4` + `kind=unauthenticated`（`C7`）——**不偷偷降级**、不读缓存、不换端点。\n")
	b.WriteString("  · `401` 与 `403` **归一**到同一个码 `4`，但 `kind` 分家（`unauthenticated` / `forbidden`）\n")
	b.WriteString("    —— §十二 `P-013` ④ 的定案：码不膨胀、语义不丢。\n")
	b.WriteString("  · `zerg-api`（8083 外部接入面）原来**无令牌即放行**（`authOK` 直接 `return true`）；\n")
	b.WriteString("    本版按 §十二 `P-007` 处置：**缺令牌拒启**，或显式 `--insecure` 且**横幅告警**。\n")
	b.WriteString("  · 脱敏（§九 M2 `R` 面）：**先脱敏后编码**；`context show` / `context ls` 一律掩码（`C6`）。\n")
	return b.String()
}
