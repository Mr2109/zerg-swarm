// tokenchain.go —— 共享令牌的**唯一解析链**（GAP-20261017-02 / GAP-20261017-04 收敛 · 单一真源）。
//
// TOKEN-CHAIN: ZERG_TOKEN > ~/.zerg/token > ZERG_AUTH_TOKEN > ZERG_API_TOKEN
//
// 即：`env:ZERG_TOKEN` > `file:~/.zerg/token` > `env:ZERG_AUTH_TOKEN`（旧名·兼容）> `env:ZERG_API_TOKEN`（旧名·兼容）。
//
// ★ **唯一实现**：本件是全仓（含另一 module `agent/`）唯一持有优先级的代码。
//
//	· 主控与 CLI：`core/internal/config` 只是转发层（`ResolveAuthTokenWithSource` → 本件）；
//	· 子端：`agent/cmd/zerg-agentd` 直接 import 本件。
//	⇒ 「三处各读一遍 = 三套优先级」那条病根被拔掉：要改顺序只改这里。
//
// ★ 旧名只**降级**、不删：它们排在文件之后，命中也只是「最后一级」，且进程内**出声一次**。
// ★ 五不进照旧：令牌**不进 argv / URL / 日志 / 聊天 / 入库**。本件只产**来源标记**，
// 任何报话都不得拼接令牌值 —— `Source` 的取值域里没有令牌值可言，`MissingHint()` 也拿不到值。
package credential

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// 令牌的三个环境变量名（真源；✗ 别在别处再抄一份字面量）。
const (
	// EnvPrimary —— 主路（UI 启动器注入的就是它）。
	EnvPrimary = "ZERG_TOKEN"
	// EnvLegacyAuth / EnvLegacyAPI —— 旧名，降级为兼容别名（保留不删）。
	EnvLegacyAuth = "ZERG_AUTH_TOKEN"
	EnvLegacyAPI  = "ZERG_API_TOKEN"
)

// Source —— 令牌取自哪一路（**只有来源，永不含令牌值**）。
type Source string

const (
	SourceEnv        Source = "env:ZERG_TOKEN"
	SourceFile       Source = "file:~/.zerg/token"
	SourceLegacyAuth Source = "env:ZERG_AUTH_TOKEN(旧名·兼容)"
	SourceLegacyAPI  Source = "env:ZERG_API_TOKEN(旧名·兼容)"
	// SourceConfig —— 兜底：fleet.yaml 的 `auth.token`（库内零明文，只作最后兼容）。
	SourceConfig Source = "config:fleet.yaml#auth.token"
	// SourceMissing —— 全空（调用方必须显式处理，不得静默放行）。
	SourceMissing Source = "缺（未配置令牌）"
)

// Chain —— **优先级链唯一字面量**（`zerg help credentials`、子端报话、判据共用）。
const Chain = string(SourceEnv) + " > " + string(SourceFile) + " > " +
	string(SourceLegacyAuth) + " > " + string(SourceLegacyAPI)

// IsLegacy —— 这一级是不是兼容旧名（降级档）。
func (s Source) IsLegacy() bool {
	return s == SourceLegacyAuth || s == SourceLegacyAPI
}

// MissingHint —— 缺令牌时的**机器可读**报话：`token_source=` 键 ＋ `TOKEN-CHAIN:` 行。
// ★ 签名即不变量：本函数拿不到令牌值 ⇒ 报话里不可能回显令牌。
func MissingHint() string {
	return "token_source=" + string(SourceMissing) + "\nTOKEN-CHAIN: " + Chain
}

// FilePath —— 共享令牌文件路径（默认 ~/.zerg/token，单行；仓库外，天然不入库）。
// 路径也只有这一份：主控 `config.TokenFilePath` 与 UI 启动器都转发到这里。
func FilePath() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".zerg", "token")
	}
	return ""
}

var (
	// legacyOnce 用指针持有，测试可整体换一枚新的（avoid copylocks）。
	legacyOnce = &sync.Once{}
	// legacySink 只在测试里换（生产恒 os.Stderr）；换它不改变「一次」的语义。
	legacySink io.Writer = os.Stderr
)

// AnnounceLegacy —— 旧名命中时**出声一次**（进程级），只说「从哪一路取到」。
// ✗ 绝不回显令牌：参数是来源标记，本函数拿不到令牌值（签名即不变量）。
func AnnounceLegacy(src Source) {
	if !src.IsLegacy() {
		return
	}
	legacyOnce.Do(func() {
		fmt.Fprintf(legacySink, "⚠ 令牌取自兼容旧名 %s —— 旧名已降级为兼容别名（保留不删）；"+
			"建议改用 %s 或 %s（令牌值不进日志）\n", src, SourceEnv, SourceFile)
	})
}

// ResolveWithSource —— **唯一解析逻辑**：返回令牌 + 来源标记。
// 纯读取：不打印、不缓存、不落盘、不写日志（报话由调用方按 `Source` 出）。
func ResolveWithSource() (string, Source) {
	if v := strings.TrimSpace(os.Getenv(EnvPrimary)); v != "" {
		return v, SourceEnv
	}
	if p := FilePath(); p != "" {
		if b, err := os.ReadFile(p); err == nil {
			if v := strings.TrimSpace(string(b)); v != "" {
				return v, SourceFile
			}
		}
	}
	if v := strings.TrimSpace(os.Getenv(EnvLegacyAuth)); v != "" {
		return v, SourceLegacyAuth
	}
	if v := strings.TrimSpace(os.Getenv(EnvLegacyAPI)); v != "" {
		return v, SourceLegacyAPI
	}
	return "", SourceMissing
}

// FromEnviron —— 在一份 `KEY=VALUE` 环境快照上按**同一条链**取名（调用方已有 env 列表时用它，
// ✗ 别自写第二套顺序：`ZERG_AUTH_TOKEN` 单名扫描曾让 `ZERG_TOKEN` 成死支）。
// 只覆盖环境变量三级；文件级由 `ResolveWithSource` 负责（快照里没有文件）。
func FromEnviron(environ []string) (string, Source) {
	vals := make(map[string]string, len(environ))
	for _, kv := range environ {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		vals[k] = v
	}
	for _, lvl := range []struct {
		name string
		src  Source
	}{
		{EnvPrimary, SourceEnv},
		{EnvLegacyAuth, SourceLegacyAuth},
		{EnvLegacyAPI, SourceLegacyAPI},
	} {
		if v := strings.TrimSpace(vals[lvl.name]); v != "" {
			return v, lvl.src
		}
	}
	return "", SourceMissing
}
