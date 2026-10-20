// inject.go — 虫茧的 AI 能力注入：**端点分配 ＋ 占位符换值 ＋ 响应擦除**（v3 的可执行形态）
//
// ★ 三条不变量（改这里先读 v3 §A/§D 与判据 6–12）：
//
//	① **真值只在宿主侧**：茧内 env 里只有回环 base 与模型 id，一个密钥字节都没有。
//	   茧内代码要访问上游，必须带**占位符**（`{{secret:<name>}}`）出门，由宿主 broker 换值。
//	② **换值两道闸，缺一不放行**：目标 host 在**上游**白名单里（管"哪一家"），且占位符只出现在
//	   **可换值 header 白名单**里（管"哪个字段"）。任一不过 ⇒ 整条请求拒（fail-closed），
//	   且**输入 map 一个字节都不改**（先全量判定，再克隆改写；✗ 不做"边判边改"）。
//	③ **错误与审计里永不出现真值**：错误串只用「密钥名 / host / header 名」三类**引用**；
//	   解析器自己的错误文本**不透传**（它可能把值拼进 message）；审计事件结构体里没有能装真值的字段。
//
// ★ 本文件**不碰网络、不碰 broker 进程**：`BuildAIEnv` 只算形状，`RewriteRequest` / `ScrubResponse`
// 是纯函数（出入口都是内存里的 map / []byte）。端口由 broker 侧 bind(":0") 后回写（v3 §C），
// 故 base 一律**由参数传入**，本件绝不自己定端口 —— 否则「端口唯一」这条就被两个出题人各写一份。
//
// ★ 为什么 host 白名单只收**上游**、不收回环：茧 → broker 那一跳若也换值，真值就重新进入茧
// 能读到的报文里，等于白做。故 `127.0.0.1` 不在默认表内（它出现在请求里只可能是被误用）。
//
// ── ⑤ 茧内拿到的三样东西（`cocoon open` 的注入面 · T4）────────────────────────
//
// `[茧.ai] enabled = true` 时，茧内服务拿到三个环境变量，**一个字节的真值都没有**：
//
//	ZERG_AI_BASE       = 宿主 broker 的回环 base（`http://127.0.0.1:<port>`，回环/无 userinfo/无 query）
//	ZERG_AI_MODEL      = 模型 id
//	ZERG_AI_TOKEN_FILE = 茧内一个**只读件**的路径，内容恒为占位符 `zerg-broker`
//
// ★ 三条不变量（照 ①②③ 的口径延伸，改这里先读它们）：
//
//	ⓐ **令牌文件里只有占位符**：`zerg-broker` 是一个**协议标记**（"请向宿主 broker 换票"），
//	  不是令牌、也不是任何令牌的前缀/哈希。真令牌既不入 env、也不入茧内任何文件。
//	ⓑ **真值只在宿主 broker 手里**：茧内要真的发一次 AI 请求，必须带着这个占位符走
//	  broker（`cocoon.Broker` 的 `ai.request` op：先 Authorize，再由 `RewriteRequest` 换值）。
//	ⓒ **本版没有 broker 的命令面** ⇒ 占位符是**终点**，不是"能用但没人验"的半成品：
//	  `zerg ai inject` / `ai call` 这一类命令面在**本仓现读不存在**（见文件头这段的出处：
//	  `grep -rn '"ai ' core/cmd/zerg` 命中 0）。⇒ 茧内拿到 base 也**调不通**上游；要真跑通
//	  必须先有 broker 进程（`NewBroker` + `Listen` + `Serve` + 令牌面）。
//	  这一段是**照实登记的未接面**，✗ 不许把它读成"已经接上了"。
package cocoon

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	// EnvAIBase — 茧内看到的上游地址：**宿主 broker 的回环 base**，不是真上游。
	EnvAIBase = "ZERG_AI_BASE"
	// EnvAIModel — 茧内使用的模型 id（与 Zerg 其余组件的 ZERG_AI_MODEL 同一口径）。
	EnvAIModel = "ZERG_AI_MODEL"
	// EnvAITokenFile — 茧内**只读占位符件**的路径。★ 文件内容恒为 AITokenPlaceholder，
	// 真令牌既不在这个文件里、也不在 env 里（见件头 ⑤）。
	EnvAITokenFile = "ZERG_AI_TOKEN_FILE"

	// AITokenPlaceholder — 记在令牌文件里的**唯一内容**：一个协议标记，不是一个令牌。
	//
	// ★ 为什么不是"空文件"或"真令牌的脱敏形"：
	//   · 空文件会让茧内程序分不清「没有令牌面」与「有令牌面但读不到」；
	//   · 放脱敏形（`abcd…wxyz`）会诱导人把它当凭据去拼 Authorization；
	//   · 放一个固定标记，茧内程序**只能**照协议把它整串交给 broker —— 它抄不出任何密钥材料。
	// ★ 也不带任何前缀/长度/哈希信息：任何"看起来像令牌"的形态都会被当成令牌去用。
	AITokenPlaceholder = "zerg-broker"

	// AITokenHostFileName — 占位符件在**茧目录**里的文件名（宿主侧真身：`<茧目录>/ai.token`）。
	AITokenHostFileName = "ai.token"
	// AITokenInJailPath — 换了根（`[茧].rootfs`）时，占位符件在**茧内**的落点。
	//
	// ★ 选 `/tmp` 下而不是 `/run` 或新建目录：`/tmp` 是 nsjail 挂载面里**唯一保证存在且可写**
	// 的目录（rootfs 档由茧目录逐茧绑进去，见 nsjailPlanMounts），bind mount 的目标父目录
	// 一定在；而 `/run` 在最小镜像（debian-slim 一类）里不保证有。★ 件本身是 **ro 绑定**，
	// 可写的是目录、不是这个件（写它得到 EROFS）。
	AITokenInJailPath = "/tmp/zerg-ai.token"

	// ScrubPlaceholder — 响应擦除后的替身。与 obs/redact 的通用遮蔽同口径（`[REDACTED]`），
	// 免得同一份日志里出现两种记号、让人以为还漏了一处。
	ScrubPlaceholder = "[REDACTED]"

	// secretPlaceholderPrefix / Suffix — 占位符定界：`{{secret:openai}}`。
	// ★ 为何不用 spec.toml 的 `secret://<name>`：那里是**整值**引用（`^secret://…$`），
	// 而 header 值是复合串（`Bearer <…>`、`Token <…>`），必须有明确的双花括号定界；
	// 否则"这一段里到底哪几个字符是钥匙"得靠猜。两种形态各有其位，不合并。
	secretPlaceholderPrefix = "{{secret:"
	secretPlaceholderSuffix = "}}"

	// minScrubLen — 短于 4 字节的真值不擦。擦 1–3 字节的串会把整份响应打成马赛克
	// （值形如 "abc" 时 HTML 里到处命中），而真密钥不可能这么短。
	minScrubLen = 4
)

var (
	// ErrAIPlanEmptyInstance — 实例 id 为空。空 id 的 plan 无从归属，broker 侧也做不成"每茧一票"。
	ErrAIPlanEmptyInstance = errors.New("cocoon: AI 计划实例 id 为空")
	// ErrAIPlanEmptyModel — 模型 id 为空（空 id 会把上游打到缺省模型上，事故面比报错大）。
	ErrAIPlanEmptyModel = errors.New("cocoon: AI 计划模型 id 为空")
	// ErrAIPlanBadModel — 模型 id 含空白/控制字符（会把 envp 打断）。
	ErrAIPlanBadModel = errors.New("cocoon: AI 计划模型 id 含空白或控制字符")
	// ErrAIPlanBadUpstream — upstream 不是可解析的绝对 URL。★ 不回显原值（可能夹带凭据）。
	ErrAIPlanBadUpstream = errors.New("cocoon: AI 计划 upstream 形态非法（不回显原值）")
	// ErrAIPlanUpstreamCredential — upstream 带 userinfo。那是凭据，**不许**进 env（进了茧就看得见）。
	ErrAIPlanUpstreamCredential = errors.New("cocoon: AI 计划 upstream 带 userinfo（凭据不进 env）")
	// ErrAIPlanUpstreamQuery — upstream 带 query/fragment。它们唯一的作用是夹带令牌，一律拒。
	ErrAIPlanUpstreamQuery = errors.New("cocoon: AI 计划 upstream 带 query/fragment")
	// ErrAIPlanNotLoopback — base 必须指向宿主回环。指到别处等于把茧内的流量放出宿主。
	ErrAIPlanNotLoopback = errors.New("cocoon: AI 计划 base 必须是宿主回环地址（127.0.0.0/8 · ::1 · localhost）")
	// ErrAIPlanBadTokenFile — 茧内占位符件的路径非法（空 / 相对 / 含空白控制字符）。
	// ★ 相对路径在茧内是「看 cwd 才知道指向哪」的串，而那正是我们要消掉的那类不确定性。
	ErrAIPlanBadTokenFile = errors.New("cocoon: AI 计划的令牌件路径非法（必须是茧内绝对路径，且不含空白/控制字符）")

	// ErrBadHost — host 形态非法（空、带路径、带 userinfo、带空白）。
	ErrBadHost = errors.New("cocoon: host 形态非法")
	// ErrHostNotAllowed — 目标 host 不在上游白名单（fail-closed：整条请求拒）。
	ErrHostNotAllowed = errors.New("cocoon: 目标 host 不在 AI 上游白名单")
	// ErrHeaderNotAllowed — 占位符出现在不可换值的 header 里。拒，而不是"留着不管"：
	// 留着会把字面量占位符发给上游（静默错配），而放开等于把"哪里能出凭据"交给上层决定。
	ErrHeaderNotAllowed = errors.New("cocoon: 占位符出现在不可换值的 header")
	// ErrBadPlaceholder — 占位符形态非法（未闭合、名字不合规）。不猜、不跳过。
	ErrBadPlaceholder = errors.New("cocoon: 占位符形态非法")
	// ErrNoSecretResolver — 有占位符却没有解析器。此时**必须**拒：原样转发等于把占位符当真值发上游。
	ErrNoSecretResolver = errors.New("cocoon: 未提供密钥解析器")
	// ErrSecretUnavailable — 解析失败。★ 不透传解析器的错误文本（它可能把真值/上游报文拼进去）。
	ErrSecretUnavailable = errors.New("cocoon: 密钥解析失败（原因不透传，防夹带真值）")
	// ErrSecretEmpty — 解析器给了空串。空串换值等于把 header 清空，且下游会当成"未认证"重试。
	ErrSecretEmpty = errors.New("cocoon: 解析器给出的密钥值为空")
)

// secretNameRe — 占位符里的密钥名，与 spec.go 的 `secret://<name>` 同字符集（少一套写法要记）。
var secretNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

// ── ① 端点分配：茧内 env 的形状 ──────────────────────────────────────────

// AIPlan — 给一个茧的 AI 指向。这是**形状**，不是连接：没有任何拨号动作发生在这一层。
//
// ★ 四个字段都不承载真值：Base 是宿主回环地址，Model 是模型 id，TokenFile 是茧内一个
// **只读占位符件**的路径（件内容恒为 AITokenPlaceholder），Env 只由这三项拼成。
type AIPlan struct {
	// Base — 宿主 broker 的回环 base（`http://127.0.0.1:<port>`，端口由 broker 回写后传入）。
	Base string
	// Model — 模型 id。
	Model string
	// TokenFile — 茧内只读占位符件的路径（空 = 本档不注入令牌件面）。
	TokenFile string
	// Env — 交给茧内的环境变量（EnvAIBase / EnvAIModel，给了 TokenFile 时再加 EnvAITokenFile）。
	Env map[string]string
}

// Valid — 形状是否成立（Base 与 Model 都非空）。零值 plan 即"没有 AI 能力"，是安全侧。
func (p AIPlan) Valid() bool { return p.Base != "" && p.Model != "" }

// String — 日志面。只有非敏感字段，故不设脱敏形。
func (p AIPlan) String() string {
	return fmt.Sprintf("AIPlan{base=%q model=%q token_file=%q}", p.Base, p.Model, p.TokenFile)
}

// GoString — `%#v` 也走日志面。
func (p AIPlan) GoString() string { return p.String() }

// WithTokenFile — 补上「茧内占位符件」这一格（返回**副本**，不改原 plan：调用方可能正在
// 并发读同一份 plan）。
//
// ★ 只接受茧内**绝对路径**：相对路径在茧内指向哪里取决于 cwd，而 cwd 来自契约 / 后端 ——
// 那等于把「去哪读令牌」交给第三方决定。空白/控制字符同样拒（会打断 envp 或 argv）。
func (p AIPlan) WithTokenFile(inCocoonPath string) (AIPlan, error) {
	if !p.Valid() {
		return AIPlan{}, ErrAIPlanEmptyInstance
	}
	s := strings.TrimSpace(inCocoonPath)
	if s == "" || !path.IsAbs(s) || hasEnvBreak(s) {
		return AIPlan{}, fmt.Errorf("%w: %q", ErrAIPlanBadTokenFile, inCocoonPath)
	}
	env := make(map[string]string, len(p.Env)+1)
	for k, v := range p.Env {
		env[k] = v
	}
	env[EnvAITokenFile] = s
	return AIPlan{Base: p.Base, Model: p.Model, TokenFile: s, Env: env}, nil
}

// Environ — 按 key 排序的 `K=V` 列表（exec 的 envp 要稳定顺序，否则两次启动的 env 哈希不同）。
func (p AIPlan) Environ() []string {
	if len(p.Env) == 0 {
		return nil
	}
	keys := make([]string, 0, len(p.Env))
	for k := range p.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, k+"="+p.Env[k])
	}
	return out
}

// BuildAIEnv — 产出给茧内的环境变量。**参数校验不过 ⇒ 零值 plan**（没有 Env ⇒ 茧内拿不到 AI 能力，
// 而不是拿到一个可疑 base）。要错误原因用 BuildAIEnvChecked。
//
//	instID   — 茧的实例 id（空 ⇒ 拒：plan 无从归属）
//	model    — 模型 id（空/含控制字符 ⇒ 拒）
//	upstream — 宿主 broker 的回环 base，如 `http://127.0.0.1:8082`（**端口由调用方给**）
func BuildAIEnv(instID, model, upstream string) AIPlan {
	plan, err := BuildAIEnvChecked(instID, model, upstream)
	if err != nil {
		emitInject(InjectEvent{Host: hostLabel(upstream), OK: false, Reason: injectReason(err)})
		return AIPlan{}
	}
	emitInject(InjectEvent{Host: hostLabel(plan.Base), OK: true, Reason: "plan"})
	return plan
}

// BuildAIEnvChecked — BuildAIEnv 的带错版本。错误串**不回显** upstream 原值（可能夹带凭据）。
func BuildAIEnvChecked(instID, model, upstream string) (AIPlan, error) {
	if strings.TrimSpace(instID) == "" {
		return AIPlan{}, ErrAIPlanEmptyInstance
	}
	m := strings.TrimSpace(model)
	if m == "" {
		return AIPlan{}, ErrAIPlanEmptyModel
	}
	if hasEnvBreak(m) {
		return AIPlan{}, ErrAIPlanBadModel
	}
	base, err := CheckAIUpstream(upstream)
	if err != nil {
		return AIPlan{}, err
	}
	return AIPlan{
		Base:  base,
		Model: m,
		Env: map[string]string{
			EnvAIBase:  base,
			EnvAIModel: m,
		},
	}, nil
}

// CheckAIUpstream — AI base 的**唯一**校验口（契约解析期与注入期共用同一份口径）。
//
// 收：`http`/`https` · 有 host · **回环** · 无 userinfo · 无 query/fragment。
// 拒：其余一切（不回显原值 —— 它可能夹带凭据）。
//
// ★ 为什么契约层也要走这里：`[茧].ai.base` 写一个 `http://user:tok@<cluster-ip>:8082`
// 就是"把令牌写进 base"，而这条路径一旦在 open 期才拒，契约已经落盘、人已经以为配好了。
// 也正因为共用同一个口，两处不可能对「什么算合法 base」各说一套。
func CheckAIUpstream(upstream string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(upstream))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", ErrAIPlanBadUpstream
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", ErrAIPlanBadUpstream
	}
	if u.User != nil {
		return "", ErrAIPlanUpstreamCredential
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return "", ErrAIPlanUpstreamQuery
	}
	if !isLoopbackHost(u.Hostname()) {
		return "", fmt.Errorf("%w: host %q", ErrAIPlanNotLoopback, u.Hostname())
	}
	return u.Scheme + "://" + u.Host + strings.TrimSuffix(u.EscapedPath(), "/"), nil
}

// ── ①b 茧内占位符件（`ZERG_AI_TOKEN_FILE` 指向的那个件）──────────────────────

// AITokenHostPath — 占位符件在**宿主侧**的落点：`<茧目录>/ai.token`。
//
// ★ 放在茧目录里（而不是状态根旁边另开一处）：删茧目录一次收干净，而且它属于"这座茧"，
// 不属于整台机 —— 两座茧各有各的 0444 占位符件，谁也改不到谁的。
func AITokenHostPath(cocoonDir string) string {
	return filepath.Join(cocoonDir, AITokenHostFileName)
}

// WriteAITokenPlaceholder — 写占位符件：内容恒为 `AITokenPlaceholder` ＋ 一个换行，权限 0444。
//
// ★ 幂等：内容是常量，同一目录重复 open 不产生内容 diff（只有 mtime 会动）。
// ★ 为什么是"临时件 ＋ rename"而不是直接 `os.WriteFile`：目标件是 **0444**，第二次写会
// EACCES（owner 也写不进去 —— 这正是我们要的最终形态）。同目录 rename 是原子的，
// 茧内程序永远读不到半截内容。
// ★ 返回值是**宿主**路径；茧内看到哪条路径由 AITokenInCocoonPath 决定（两者在没换根时相同）。
func WriteAITokenPlaceholder(cocoonDir string) (string, error) {
	if strings.TrimSpace(cocoonDir) == "" {
		return "", fmt.Errorf("%w: 茧目录为空", ErrAIPlanBadTokenFile)
	}
	if err := os.MkdirAll(cocoonDir, 0o700); err != nil {
		return "", fmt.Errorf("cocoon: 建茧目录 %s 失败: %w", cocoonDir, err)
	}
	host := AITokenHostPath(cocoonDir)
	fh, err := os.CreateTemp(cocoonDir, ".ai.token-*")
	if err != nil {
		return "", fmt.Errorf("cocoon: 建占位符临时件失败: %w", err)
	}
	tmp := fh.Name()
	// 失败路径一律把临时件删掉：留一个 `.ai.token-*` 在茧目录里会被误读成"有人动过令牌面"。
	cleanup := func() { _ = os.Remove(tmp) }
	if _, werr := fh.WriteString(AITokenPlaceholder + "\n"); werr != nil {
		_ = fh.Close()
		cleanup()
		return "", fmt.Errorf("cocoon: 写占位符件失败: %w", werr)
	}
	// ★ 先 chmod 再 close：rename 之后就没法（也不该）再改权限位了。
	if cerr := fh.Chmod(0o444); cerr != nil {
		_ = fh.Close()
		cleanup()
		return "", fmt.Errorf("cocoon: 占位符件设 0444 失败: %w", cerr)
	}
	if cerr := fh.Close(); cerr != nil {
		cleanup()
		return "", fmt.Errorf("cocoon: 关占位符临时件失败: %w", cerr)
	}
	if rerr := os.Rename(tmp, host); rerr != nil {
		cleanup()
		return "", fmt.Errorf("cocoon: 占位符件落位 %s 失败: %w", host, rerr)
	}
	return host, nil
}

// ReadAITokenPlaceholder — 读回占位符件的内容（排障与判据用）。
//
// ★ 调用方**不许**把它当"读到了令牌"：它恒为 AITokenPlaceholder；真值不在这里，也不该
// 有代码去别处找它（真值的唯一来源是宿主 broker 的 SecretResolver）。
func ReadAITokenPlaceholder(hostPath string) (string, error) {
	b, err := os.ReadFile(hostPath)
	if err != nil {
		return "", fmt.Errorf("cocoon: 读占位符件 %s 失败: %w", hostPath, err)
	}
	return strings.TrimRight(string(b), "\n"), nil
}

// AITokenInCocoonPath — 占位符件在**茧内**看到的路径。
//
//	没换根（chroot = `/`）⇒ 茧内就是宿主文件系统视图，宿主路径原样可用（0444 即"只读件"）。
//	换了根（`[茧].rootfs`/overlay）⇒ 茧内看到的是 merged 视图，宿主路径在里面不存在
//	  ⇒ 用固定落点 AITokenInJailPath，由 AITokenMount 以 **ro bind** 挂进去。
//
// ★ 这里**不猜**：换没换根是调用方（命令面）从契约读出来的事实，不由本函数去嗅探盘面。
func AITokenInCocoonPath(chrooted bool, hostPath string) string {
	if chrooted {
		return AITokenInJailPath
	}
	return hostPath
}

// AITokenMount — 需要时给出「把占位符件以只读方式挂进茧」的那条挂载项。
//
// 返回 ok=false ⇒ 不需要挂（没换根，宿主路径在茧内本来就可见）。这**不是**失败：
// 换根与否决定"可见性从哪来"，两种都只给一个**只读**的占位符件。
func AITokenMount(chrooted bool, hostPath string) (OrchestrateMount, bool) {
	if !chrooted {
		return OrchestrateMount{}, false
	}
	return OrchestrateMount{Host: hostPath, Inner: AITokenInJailPath, RO: true}, true
}

// CocoonChroots — 这份契约**会不会换根**（命令面问它，✗ 别自己去看 rootfs 串）。
//
// ★ 单一口径：直接落到真正决定 chroot 的那一处（`nsjailChroots`，`nsjailRootfsPlanFor`
// 的早退也用它）—— 复制的第二份「rootfs 非空且 ≠ /」迟早分家，而分家的代价是
// "按宿主路径给茧内一个读不到的令牌件路径"（看着有、其实读不到）。
func CocoonChroots(spec *Spec) bool {
	return nsjailChroots(spec)
}

// hasEnvBreak — env 值里出现空白/控制字符就会把 envp 打断（`K=V W` 里多出一个变量名）。
func hasEnvBreak(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return strings.ContainsAny(s, " \t")
}

// isLoopbackHost — 纯字面量判定，**不做 DNS**：解析一次就能被 /etc/hosts 骗到非回环地址上。
func isLoopbackHost(h string) bool {
	if strings.EqualFold(h, "localhost") {
		return true
	}
	ip := net.ParseIP(strings.Trim(h, "[]"))
	return ip != nil && ip.IsLoopback()
}

// hostLabel — 只取 host（不带 userinfo/路径/端口），供审计与错误串使用；解析不了给空串。
func hostLabel(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

// ── ② 占位符换值 ──────────────────────────────────────────────────────

// SecretResolver — 密钥真值的**唯一**来源。broker 侧持有实现（接宿主密钥库）。
// ★ 茧内任何进程都不该有实现：接口在这是为了让 broker 注入，"可替换"不是"可下放"。
type SecretResolver interface {
	// Resolve — 按键名取真值。返回值只有调用点用，不进日志、不进错误串。
	Resolve(name string) (string, error)
}

// FuncSecretResolver — 函数适配器（broker 接入与测试两用）。
type FuncSecretResolver func(name string) (string, error)

// Resolve — nil 函数一律拒（fail-closed，✗ 不 panic）。
func (f FuncSecretResolver) Resolve(name string) (string, error) {
	if f == nil {
		return "", ErrNoSecretResolver
	}
	return f(name)
}

// SecretPlaceholder — 生成 `{{secret:<name>}}`。茧内代码必须用本函数拼占位符，
// 免得两处手写花括号、把一个拼错成四个。
func SecretPlaceholder(name string) string {
	return secretPlaceholderPrefix + name + secretPlaceholderSuffix
}

// AIHeaderAllowlist — 允许承载真值的 header（大小写不敏感）。
// ★ 只收认证类：它们被中间件重写的语义是明确的；把真值往任意 header 里塞，等于把
// "哪个字段会带凭据出门"交给上游的日志/错误页决定。
func AIHeaderAllowlist() []string {
	return []string{"Authorization", "Proxy-Authorization", "X-Api-Key", "Api-Key", "X-Goog-Api-Key"}
}

// DefaultAIHostAllowlist — 默认放行的**上游** host（不含端口，大小写不敏感）。
// ★ 回环地址**不在**表内：茧 → broker 那一跳不许换值，理由见件头第三条星标。
func DefaultAIHostAllowlist() []string {
	return []string{
		"api.openai.com",
		"api.anthropic.com",
		"generativelanguage.googleapis.com",
		"api.deepseek.com",
	}
}

// RewriteRequest — 把发往白名单 host 的请求头里的占位符换成真值。用默认上游白名单。
//
// 约定（调用方必须照做）：
//   - 返回 err ⇒ **整条请求不许发**（fail-closed）；此时 headers 未被修改，返回值是 nil。
//   - 返回成功 ⇒ headers2 是**新 map**（输入不受影响，可安全用于并发复用）。
func RewriteRequest(headers http.Header, host string, secrets SecretResolver) (http.Header, error) {
	return RewriteRequestWithAllowlist(headers, host, DefaultAIHostAllowlist(), secrets)
}

// RewriteRequestWithAllowlist — 可指定上游白名单的版本（broker 侧接自建上游时用）。
//
// 顺序是刻意的：先判 host（与载荷无关的硬闸）→ 再全量扫占位符（只判不改）→ 最后克隆改写。
// 这样"某一条 header 不合法"不会留下改了一半的 map —— 半成品 map 比原样更危险。
func RewriteRequestWithAllowlist(headers http.Header, host string, hostAllowlist []string, secrets SecretResolver) (http.Header, error) {
	h, err := normalizeHost(host)
	if err != nil {
		emitInject(InjectEvent{Host: host, OK: false, Reason: injectReason(err)})
		return nil, err
	}
	if !hostAllowed(h, hostAllowlist) {
		emitInject(InjectEvent{Host: h, OK: false, Reason: "host_not_allowed"})
		return nil, fmt.Errorf("%w: %q", ErrHostNotAllowed, h)
	}

	// —— 第一遍：只扫不改。map 迭代顺序不稳定，故键名先排序，产物可复现。
	type hit struct {
		key   string
		idx   int
		names []string
	}
	keys := make([]string, 0, len(headers))
	for k := range headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var hits []hit
	for _, k := range keys {
		for i, v := range headers[k] {
			if !strings.Contains(v, secretPlaceholderPrefix) {
				continue
			}
			if !headerRewritable(k) {
				emitInject(InjectEvent{Host: h, Header: k, OK: false, Reason: "header_not_allowed"})
				return nil, fmt.Errorf("%w: header %q", ErrHeaderNotAllowed, k)
			}
			names, perr := placeholderNames(v)
			if perr != nil {
				emitInject(InjectEvent{Host: h, Header: k, OK: false, Reason: "bad_placeholder"})
				return nil, fmt.Errorf("%w: header %q", ErrBadPlaceholder, k)
			}
			hits = append(hits, hit{key: k, idx: i, names: names})
		}
	}
	if len(hits) == 0 {
		// 没有占位符 ⇒ 不改、不碰解析器（nil 解析器也照常放行：这一跳根本没有换值需求）。
		return cloneHeader(headers), nil
	}
	if secrets == nil {
		emitInject(InjectEvent{Host: h, OK: false, Reason: "no_resolver"})
		return nil, fmt.Errorf("%w: host %q", ErrNoSecretResolver, h)
	}

	// —— 第二遍：解析。同名只解析一次（少一次密钥库往返，也少一条可被计时的口子）。
	vals := make(map[string]string, len(hits))
	for _, ht := range hits {
		for _, name := range ht.names {
			if _, ok := vals[name]; ok {
				continue
			}
			v, rerr := resolveSecret(secrets, name)
			if rerr != nil {
				emitInject(InjectEvent{Host: h, Header: ht.key, Name: name, OK: false, Reason: injectReason(rerr)})
				return nil, fmt.Errorf("%w: 密钥 %q（host %q）", rerr, name, h)
			}
			vals[name] = v
		}
	}

	// —— 第三遍：克隆后改写（输入永不就地修改）。
	out := cloneHeader(headers)
	for _, ht := range hits {
		out[ht.key][ht.idx] = replacePlaceholders(out[ht.key][ht.idx], vals)
		emitInject(InjectEvent{Host: h, Header: ht.key, Name: strings.Join(ht.names, ","), N: len(ht.names), OK: true, Reason: "rewritten"})
	}
	return out, nil
}

// resolveSecret — 解析 + 两条口径收敛（错误文本不透传 / 空值当失败）。
func resolveSecret(r SecretResolver, name string) (string, error) {
	v, err := r.Resolve(name)
	if err != nil {
		// ★ 不 fmt.Errorf("%w: %v", ErrSecretUnavailable, err)：解析器的 message 可能拼了真值。
		return "", ErrSecretUnavailable
	}
	if v == "" {
		return "", ErrSecretEmpty
	}
	return v, nil
}

// placeholderNames — 严格扫出占位符里的密钥名。未闭合 / 名字不合规 ⇒ 错（不猜、不跳过）。
func placeholderNames(v string) ([]string, error) {
	var names []string
	for i := 0; i < len(v); {
		j := strings.Index(v[i:], secretPlaceholderPrefix)
		if j < 0 {
			break
		}
		start := i + j + len(secretPlaceholderPrefix)
		k := strings.Index(v[start:], secretPlaceholderSuffix)
		if k < 0 {
			return nil, ErrBadPlaceholder
		}
		name := v[start : start+k]
		if !secretNameRe.MatchString(name) {
			return nil, ErrBadPlaceholder
		}
		names = append(names, name)
		i = start + k + len(secretPlaceholderSuffix)
	}
	if len(names) == 0 {
		return nil, ErrBadPlaceholder
	}
	return names, nil
}

// replacePlaceholders — 逐段替换（✗ 不用 ReplaceAllString：它会把 `$` 当捕获组展开，
// 真值里出现 `$1` 就会静默吃掉一段 —— 与 obs/redact 的 replaceLiteral 同一条教训）。
func replacePlaceholders(v string, vals map[string]string) string {
	var b strings.Builder
	b.Grow(len(v))
	for i := 0; i < len(v); {
		j := strings.Index(v[i:], secretPlaceholderPrefix)
		if j < 0 {
			b.WriteString(v[i:])
			break
		}
		start := i + j + len(secretPlaceholderPrefix)
		k := strings.Index(v[start:], secretPlaceholderSuffix)
		if k < 0 { // 已校验过，走不到这；保守起见整段保留，绝不吞字节
			b.WriteString(v[i:])
			break
		}
		name := v[start : start+k]
		val, ok := vals[name]
		if !ok {
			b.WriteString(v[i:])
			break
		}
		b.WriteString(v[i : i+j])
		b.WriteString(val)
		i = start + k + len(secretPlaceholderSuffix)
	}
	return b.String()
}

// headerRewritable — header 名是否在可换值白名单（大小写不敏感）。
func headerRewritable(key string) bool {
	for _, a := range AIHeaderAllowlist() {
		if strings.EqualFold(key, a) {
			return true
		}
	}
	return false
}

// hostAllowed — host 是否在白名单（大小写不敏感，允许白名单条目自带端口）。
func hostAllowed(host string, allowlist []string) bool {
	for _, a := range allowlist {
		n, err := normalizeHost(a)
		if err != nil {
			continue
		}
		if n == host {
			return true
		}
	}
	return false
}

// normalizeHost — 归一化目标 host：小写、去端口、去尾点、去 IPv6 方括号。
// ★ 只做**字面量**归一，不做后缀匹配：`api.openai.com.evil.com` 必须**不**命中白名单
// （后缀匹配是这一类白名单最经典的翻车点）。
func normalizeHost(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", fmt.Errorf("%w: 空 host", ErrBadHost)
	}
	if strings.ContainsAny(s, "/?#@ \t\r\n") {
		return "", fmt.Errorf("%w: 含路径/查问/userinfo/空白", ErrBadHost)
	}
	host := s
	if strings.HasPrefix(s, "[") {
		i := strings.Index(s, "]")
		if i <= 1 {
			return "", fmt.Errorf("%w: IPv6 方括号未闭合", ErrBadHost)
		}
		host = s[1:i]
		if rest := s[i+1:]; rest != "" && !strings.HasPrefix(rest, ":") {
			return "", fmt.Errorf("%w: IPv6 后方形态非法", ErrBadHost)
		}
	} else if strings.Count(s, ":") == 1 {
		host = s[:strings.Index(s, ":")]
	}
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if host == "" {
		return "", fmt.Errorf("%w: 空 host", ErrBadHost)
	}
	return host, nil
}

// cloneHeader — 深拷贝（值切片也拷）。返回值与输入不共享任何底层数组 —— 调用方能安全并发读。
func cloneHeader(in http.Header) http.Header {
	out := make(http.Header, len(in))
	for k, vs := range in {
		out[k] = append([]string(nil), vs...)
	}
	return out
}

// ── ③ 响应擦除 ────────────────────────────────────────────────────────

// ScrubResponse — 把响应体里**回显的真值**换成 `[REDACTED]`。
//
// 三种编码都要擦，缺一种就等于没擦 —— 上游的报错页/调试页经常把 Authorization 原样
// base64 或 url-encode 回来：
//
//	明文       sk-live-…
//	base64     std / raw / url / raw-url 四个变体（padding 有无、`+/` 与 `-_` 都要）
//	percent    QueryEscape（空格→`+`）与 PathEscape（空格→`%20`）
//
// ★ 本函数只做"按**已知**真值比对"，不做形态推断：推断型脱敏是 obs/redact 的活，
// 两者不互相替代（一个知道值、一个猜形态；知道值的更准，猜形态的能覆盖没登记的值）。
// ★ 空 secrets / 短于 4 字节的真值直接跳过；nil body 原样返回（不 panic、不造空切片）。
func ScrubResponse(body []byte, secrets []string) []byte {
	if len(body) == 0 || len(secrets) == 0 {
		return body
	}
	for _, v := range scrubVariants(secrets) {
		body = bytes.ReplaceAll(body, []byte(v), []byte(ScrubPlaceholder))
	}
	return body
}

// scrubVariants — 一个真值展开成全部编码变体（去重、长的先擦）。
// 长的先擦：一条真值是另一条的前缀时，先擦短的会留下尾巴碎片。
func scrubVariants(secrets []string) []string {
	seen := make(map[string]struct{}, len(secrets)*7)
	out := make([]string, 0, len(secrets)*7)
	add := func(s string) {
		if s == "" {
			return
		}
		if _, dup := seen[s]; dup {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	for _, s := range secrets {
		if len(s) < minScrubLen {
			continue
		}
		raw := []byte(s)
		add(s)
		add(base64.StdEncoding.EncodeToString(raw))
		add(base64.RawStdEncoding.EncodeToString(raw))
		add(base64.URLEncoding.EncodeToString(raw))
		add(base64.RawURLEncoding.EncodeToString(raw))
		add(url.QueryEscape(s))
		add(url.PathEscape(s))
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i]) != len(out[j]) {
			return len(out[i]) > len(out[j])
		}
		return out[i] < out[j]
	})
	return out
}

// ── ④ 审计面：只收引用，不收真值 ──────────────────────────────────────

// InjectEvent — 一次注入判定的事实。★ 结构体里**没有**能装真值的字段：
// Host/Header 是目的地，Name 是密钥**名**（引用），N 是换值个数，Reason 是分类标签。
// 这是结构性保证 —— 不是"记得别写进去"的纪律（同 auth.go 的 tokenEntry 口径）。
type InjectEvent struct {
	At     time.Time
	Host   string
	Header string
	Name   string
	N      int
	OK     bool
	Reason string
}

// String — 日志面。所有字段都是引用/计数，故无需脱敏形。
func (e InjectEvent) String() string {
	at := e.At
	if at.IsZero() {
		at = time.Now()
	}
	return fmt.Sprintf("[cocoon-inject] %s host=%q header=%q name=%q n=%d ok=%t reason=%q",
		at.UTC().Format(time.RFC3339Nano), e.Host, e.Header, e.Name, e.N, e.OK, e.Reason)
}

// GoString — `%#v` 也走日志面。
func (e InjectEvent) GoString() string { return e.String() }

var injectHook struct {
	mu sync.RWMutex
	fn func(InjectEvent)
}

// SetInjectAuditHook — 挂审计钩子（nil = 不记）。钩子在**锁外**调用，且只拿到 InjectEvent
// （拿不到真值，因为结构体里没有）。本文件自身不写任何日志（无 log/slog 依赖）。
func SetInjectAuditHook(hook func(InjectEvent)) {
	injectHook.mu.Lock()
	injectHook.fn = hook
	injectHook.mu.Unlock()
}

// emitInject — 投递审计事件（nil 钩子即静默丢弃）。
func emitInject(ev InjectEvent) {
	injectHook.mu.RLock()
	fn := injectHook.fn
	injectHook.mu.RUnlock()
	if fn == nil {
		return
	}
	if ev.At.IsZero() {
		ev.At = time.Now()
	}
	fn(ev)
}

// injectReason — 错误 → 固定分类标签。★ 只吐字面量，绝不把 err.Error() 原样带进审计
// （错误串已被约束成不含真值，但"约束"不如"不传递"稳）。
func injectReason(err error) string {
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, ErrAIPlanEmptyInstance):
		return "empty_instance"
	case errors.Is(err, ErrAIPlanEmptyModel):
		return "empty_model"
	case errors.Is(err, ErrAIPlanBadModel):
		return "bad_model"
	case errors.Is(err, ErrAIPlanBadUpstream):
		return "bad_upstream"
	case errors.Is(err, ErrAIPlanUpstreamCredential):
		return "upstream_credential"
	case errors.Is(err, ErrAIPlanUpstreamQuery):
		return "upstream_query"
	case errors.Is(err, ErrAIPlanNotLoopback):
		return "not_loopback"
	case errors.Is(err, ErrBadHost):
		return "bad_host"
	case errors.Is(err, ErrHostNotAllowed):
		return "host_not_allowed"
	case errors.Is(err, ErrHeaderNotAllowed):
		return "header_not_allowed"
	case errors.Is(err, ErrBadPlaceholder):
		return "bad_placeholder"
	case errors.Is(err, ErrNoSecretResolver):
		return "no_resolver"
	case errors.Is(err, ErrSecretEmpty):
		return "secret_empty"
	case errors.Is(err, ErrSecretUnavailable):
		return "secret_unavailable"
	}
	return "unknown"
}
