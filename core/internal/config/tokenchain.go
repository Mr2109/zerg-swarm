// tokenchain.go —— 令牌链的**转发层**（GAP-20261017-02 / GAP-20261017-04 收敛 · 单一真源）。
//
// TOKEN-CHAIN: ZERG_TOKEN > ~/.zerg/token > ZERG_AUTH_TOKEN > ZERG_API_TOKEN
//
// ★ 本件**不持有优先级**：实现只有一份，在共享模块 `shared/credential/tokenchain.go`
// （子端在另一 module `agent/`，import 不了 `core/internal/**` ⇒ 真源必须落在两边都
// import 得到的 `shared/`）。这里只做「名字不变 ＋ 一个函数转发」，既有调用面零改动。
// 🚫 别在此处补第二套「自读环境变量」：三处各读一遍 = 三套优先级，正是本单要拔的病根。
// ★ 五不进照旧：令牌**不进 argv / URL / 日志 / 聊天 / 入库**。本层只经手来源标记，
// 任何报话都不得拼接令牌值（`zerg context ls` 的 `token_source` 列即来源标记）。
package config

import "github.com/Mr2109/zerg-swarm/shared/credential"

// TokenSource —— 令牌取自哪一路（与真源同一个类型；**只有来源，永不含令牌值**）。
type TokenSource = credential.Source

const (
	// 令牌的三个环境变量名（转发真源常量；✗ 别在本仓别处再抄一份字面量）。
	TokenEnvPrimary      = credential.EnvPrimary
	TokenLegacyAuthToken = credential.EnvLegacyAuth
	TokenLegacyAPIToken  = credential.EnvLegacyAPI

	TokenSourceEnv        = credential.SourceEnv
	TokenSourceFile       = credential.SourceFile
	TokenSourceLegacyAuth = credential.SourceLegacyAuth
	TokenSourceLegacyAPI  = credential.SourceLegacyAPI
	// TokenSourceConfig —— 兜底：fleet.yaml 的 `auth.token`（库内零明文，只作最后兼容）。
	TokenSourceConfig = credential.SourceConfig
	// TokenSourceMissing —— 三路都空（调用方必须显式处理，不得静默放行）。
	TokenSourceMissing = credential.SourceMissing

	// TokenChain —— **优先级链唯一字面量**（`zerg help credentials` 与来源报话共用）。
	// 对拍判据按这一串逐字核各读取面。
	TokenChain = credential.Chain
)

// ResolveAuthTokenWithSource —— **唯一解析逻辑的转发**：真实现 = `credential.ResolveWithSource`。
// 纯读取：不打印、不缓存、不落盘、不写日志（报话由调用方按 `TokenSource` 出）。
func ResolveAuthTokenWithSource() (string, TokenSource) {
	return credential.ResolveWithSource()
}

// AnnounceLegacyTokenSource —— 旧名命中时**出声一次**（进程级），只说「从哪一路取到」。
// ✗ 绝不回显令牌：参数是来源标记，本函数拿不到令牌值（签名即不变量）。
func AnnounceLegacyTokenSource(src TokenSource) {
	credential.AnnounceLegacy(src)
}

// TokenFromEnviron —— 在一份 `KEY=VALUE` 环境快照上按**同一条链**取名（转发真源，✗ 不另立顺序）。
func TokenFromEnviron(environ []string) string {
	v, _ := credential.FromEnviron(environ)
	return v
}
