// family_cocoon_lifecycle.go —— 虫茧沙箱的生命周期命令面（C1）：
//
//	zerg cocoon open <dir> [--dry-run] [--json <字段>]
//	zerg cocoon close <id> [--purge] [--confirm=<id>] [--json <字段>]
//	zerg cocoon ls [--json <字段>]
//	zerg cocoon inspect <id> [--json <字段>]
//	zerg cocoon bind <id> <卵名> [--json <字段>]
//
// ★ 与仓内既有 `cocoon pack|info|verify|xfer`（A12 归名 · 卵的单文件封装 · 真搬体在 `zerg_egg.go`）
// **不是一件事**：那四条是「卵包」工具，本件是「茧」的运行期生命周期。四条一个字节不碰。
//
// 口径（逐条都是判据，不是文风）：
//
//	① **真源是 meta.json**（`core/internal/cocoon/meta.go`），不是目录存在：
//	   `ls` 只列 meta 能读出来的茧；目录在而 meta 没有 ⇒ 孤儿，**本件不列、不删**
//	   （那是 reconcile 的活 —— 列出来等于把「有目录」冒充成「有茧」）。
//	② **路径入参先 realpath 再校验允许根**（`cocoonAllowedRoots`）：`open <dir>` 的目录
//	   必须落在允许根内，`<id>` 一律先过 `cocoonValidID`（不许 `.`/`..`/路径分隔符）。
//	③ **非 TTY 不下提示**：全部靠旗标（`--dry-run` / `--purge` / `--confirm=<id>`）。
//	④ **退出码闭集 0/1/2**（本件不新开号；`failed`=1 · `usage`=2 —— 闭集外一律不用）。
//	⑤ **先写 meta 再动**：open 先落 created→starting 再 Provision/起服务；close 先落
//	   stopping 再停；`--purge` 先留墓碑再删（`cocoon.RemoveCocoon` 内部保证「墓碑失败 ⇒ 零删除」）。
//	⑥ **后端如实署名**：`backend=<meta.backend>`（fake / nsjail），能力面照后端自报的
//	   `cocoon.Capabilities` 真值印（弱项逐条写出来，✗ 不冒充 strong；后端不报能力面时
//	   如实印「不可用」而不是填一张全 false 的表）。
//	⑦ **契约哈希锁（判据 9 · v2 §E）**：`open` 把 `<dir>/cocoon.toml` 的**原始字节** sha256
//	   落进 meta 的 `spec_sha`（连 `spec_dir` 一起 —— `inspect` 只有 id，靠它回读契约）；
//	   同一目录已有**未到终态**的茧时，现读字节与锁不符 ⇒ `open` 拒（零副作用，`--dry-run` 同拒）；
//	   `inspect` 只读比对并报 `ok|drift|unlocked|unreadable`，**rc 恒 0**（观察面不拒）。
//	   锁的全部实现在 `internal/cocoon/speclock.go`（只算哈希、不写盘）。
//
// 后端面（本单新接）：
//
//	`ZERG_COCOON_BACKEND` 三档：`fake`（缺省 · 脚手架，进程内记账）· `nsjail`（真后端）· `auto`
//	（linux ⇒ nsjail，其余 ⇒ fake）。**只有 `auto` 允许换档**；显式写 `nsjail` 而本平台不是
//	linux ⇒ 拒执（退 1，判词含 `ErrBackendUnsupported` 的原话），✗ 不静默降级成假后端 ——
//	那会把「没有隔离」冒充成「有隔离」。
//
//	端口：调不调 `Expose` 只看后端自报的 `Capabilities.Expose`（✗ 不按平台名猜）。linux/nsjail
//	不映射端口（茧与调用方同宿主网络栈）⇒ 契约的 `[[端口]]` **跳过并在 stderr 说明**，
//	readiness 探测改直连 `127.0.0.1:<茧内端口>`。
//
//	收尾：`open` 把后端**签发的**句柄（实例 id + pid 表）落进 meta；`close` 是另一个进程，
//	内存记账早没了 ⇒ 走 `HandleReaper.ReapPersisted`（后端自己验 pid 归属再下刀）。
//
//	rootfs：`open`/`close` 都在 Provision/Destroy 之前把**茧目录**（真源根下的 `<id>/`）交给后端
//	（`cocoon.CocoonDirSetter.SetCocoonDir`）⇒ 契约 `[茧].rootfs` 有值时按 overlay 换根
//	（lower=该目录只读 · upper/work/merged 都落在茧目录内 · 实例根也在里面 ⇒ 删茧目录一次收干净）。
//
// AI 注入面（T4 · 契约 `[茧.ai]` 可选段）：
//
//	`enabled = true` ⇒ `open` 给茧内注入三个 env，**一个字节的真值都没有**：
//
//	  ZERG_AI_BASE       = 宿主 broker 的回环 base（`[茧.ai].base` ＞ `ZERG_AI_BASE` ＞ 内置回环
//	                       `http://` ＋ `cocoon.ResolveBrokerAddr()`）；★ 缺省这一级与 broker 的
//	                       监听地址**同一个真源** —— `ZERG_BROKER_ADDR` 一处旋钮同时改两处
//	  ZERG_AI_MODEL      = `[茧.ai].model`（缺省 Nemotron-3.5-Super-VL-120B-A12B）
//	  ZERG_AI_TOKEN_FILE = 茧内一个**只读件**的路径，内容恒为占位符 `zerg-broker`
//
//	★ 三条口径（都是判据）：
//	  ① **真令牌不写盘、不入 env、不进 meta**：真值只在宿主 broker 的 SecretResolver 手里。
//	  ② 令牌件落在**茧目录**（`<茧目录>/ai.token` · 0444）；换了根（`[茧].rootfs`）时再以
//	     **ro bind** 挂到茧内 `/tmp/zerg-ai.token`（宿主路径在 merged 视图里不存在 ⇒ 必须挂，
//	     而挂载只认"起进程时给"，所以走 OrchestrateOptions.Mounts）。
//	  ③ `base` 在契约解析期就过 `CheckAIUpstream`（回环 / 无 userinfo / 无 query）⇒
//	     "把令牌写进 base"这种形态**写不进契约**，也就到不了 env。
//
// 未接的面（照实登记 · 不当已做）：
//
//	· 镜像准备（OCI 拉取/解包）：契约 `[茧].rootfs` 指的那个目录得先备好 —— 后端只报「读不到 /
//	  不是目录」，✗ 不假装我们替你拉了镜像；上层盘还必须支持叠层（tmpfs/ext4/xfs 可以，
//	  overlay-on-overlay 不行，见 core/internal/cocoon/backend_nsjail_rootfs.go）。
//	· 伪终端（ExecOpts.TTY=true 一律拒）· VZ/HCS 两个后端 · 令牌真值落点（本版只签发并在输出里给脱敏形）。
//	· **broker 的命令面**（T4c 已接）：`zerg cocoon broker <start|stop|status>` 在宿主上真起/收/看
//	  那个 `cocoon.Broker`（`NewBroker` ＋ `Listen`（unix 面）＋ `ListenHTTP`（回环面）＋ `Serve`），
//	  落件 `family_cocoon_broker.go`。⇒ 茧内 `curl $ZERG_AI_BASE/health` 的**前置条件**从此可满足：
//	  宿主先 `zerg cocoon broker start`（缺省 `127.0.0.1:8092`，与 `zerg-core` 网关的 8082 不再撞车）。
//	  ★ 仍未接的面照实登记：**broker 的多茧共享**（本版一个宿主一份 broker、`/health` 的
//	  `token_mode` 全局一格）与**per-cocoon 监听**（见 `broker_http.go` 的「未接的面」）。
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/cocoon"
	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

const (
	// cocoonRootEnv —— 茧状态根（`cocoons/<id>/meta.json` 的父目录）。缺省 = `<状态目录>/cocoons`。
	cocoonRootEnv = "ZERG_COCOON_ROOT"
	// cocoonAllowRootsEnv —— **额外**允许的 `open <dir>` 根（`:` 分隔）。默认允许根见 `cocoonAllowedRoots`。
	cocoonAllowRootsEnv = "ZERG_COCOON_ALLOW_ROOTS"
	// cocoonBackendEnv —— 后端选择：`fake`（缺省）· `nsjail` · `auto`（linux⇒nsjail，其余⇒fake）。
	cocoonBackendEnv = "ZERG_COCOON_BACKEND"
	// cocoonTokenEnv —— `inspect` 的令牌面：只回**脱敏形**（真值不进 stdout / argv / 日志）。
	cocoonTokenEnv = "ZERG_COCOON_TOKEN"
	// cocoonAIBaseEnv —— 注入面的宿主 broker 回环 base 的 env 名。
	//
	// ★ env 名是 `ZERG_AI_BASE`，**不是** `ZERG_AI_UPSTREAM`：后者已归 broker 用（`cocoon.EnvAIUpstream`
	// = 真上游地址，缺省 `http://127.0.0.1:9400`）。两个名字混用等于让宿主想「把 broker 指向模型」
	// 的那条配置顺手把**茧内的 base 也带走** —— 茧就绕过 broker 直连模型，占位符那一跳白做。
	//
	// ★ 缺省那一级**没有**本件自己的常量：它是 `cocoonDefaultAIUpstream()`（下面的函数），
	//   真源 = `cocoon.ResolveBrokerAddr` ／ `cocoon.BrokerAIBase` —— 与 broker 的监听地址同一处。
	//   （T4 是「两处各写一份 8082」，T4b 又撞上 `zerg-core` 网关；T4c 起只留**一个旋钮**：
	//   `ZERG_BROKER_ADDR`，旧名 `ZERG_BROKER_HTTP_ADDR` 当同义回退。）
	cocoonAIBaseEnv = "ZERG_AI_BASE"

	cocoonBackendFake   = "fake"
	cocoonBackendNsjail = "nsjail"
	cocoonBackendAuto   = "auto"
	// cocoonIDHexBytes —— 茧 id 的随机尾巴字节数（4 字节 ⇒ 8 位十六进制）。
	cocoonIDHexBytes = 4
	// cocoonDash —— 人面「没有」的那一格（✗ 不印空串冒充值）。
	cocoonDash = "—"
)

// cocoonLsFields / cocoonLsDisplay —— `ls` 的机器面字段与行式三格（id/phase/端口/绑定的卵）。
var (
	cocoonLsFields  = []string{"id", "phase", "ports", "egg", "pid", "created_at"}
	cocoonLsDisplay = []string{"id", "phase", "ports", "egg"}

	cocoonOpenFields = []string{"id", "phase", "pid", "ports", "egg", "backend", "token", "dir", "logs"}

	cocoonCloseFields = []string{"id", "phase", "purged", "deleted_files", "deleted_dirs",
		"tombstone", "verified", "dir", "reaped", "port_verified"}

	cocoonInspectFields = []string{"id", "phase", "pid", "ports", "egg", "owner", "created_at",
		"start_time", "backend", "backend_instance", "platform", "isolation", "peer_identity", "peer_mechanism",
		"process_tree_kill", "kill_mechanism", "symlink_nofollow", "cross_mount_guard",
		"expose", "expose_mechanism", "caps_error",
		"ai_base", "ai_model", "ai_env", "token", "token_source",
		// 判据 9（v2 §E）契约哈希锁三格：判定（ok|drift|unlocked|unreadable）· 锁定值 · 现值。
		"spec_lock", "spec_sha", "spec_now", "dir"}

	cocoonBindFields = []string{"id", "egg", "phase", "dir"}
)

// ── 状态根 / 允许根 ──────────────────────────────────────────────────────

// cocoonRoot —— 茧状态根：`ZERG_COCOON_ROOT` ＞ `<状态目录>/cocoons`。
func cocoonRoot() string {
	if v := strings.TrimSpace(os.Getenv(cocoonRootEnv)); v != "" {
		return filepath.Clean(v)
	}
	return filepath.Join(statepath.Dir(), "cocoons")
}

// cocoonAllowedRoots —— `open <dir>` 的允许根（realpath 后按**路径段**比，见 `pathWithinDir`）。
//
// 三条天然根：茧状态根（茧目录自己）· 状态目录 · 仓根；外加 `ZERG_COCOON_ALLOW_ROOTS`（`:` 分隔）。
// 为什么要有这张表：`open` 会按契约起服务、写 meta —— 让任意路径（`/`、`~/.ssh`）当「茧目录」
// 是把「路径入参」当成了信任状。
func cocoonAllowedRoots() []string {
	roots := []string{cocoonRoot(), statepath.Dir()}
	if wr := strings.TrimSpace(statepath.WorkspaceRoot()); wr != "" {
		roots = append(roots, wr)
	}
	for _, p := range strings.Split(os.Getenv(cocoonAllowRootsEnv), string(os.PathListSeparator)) {
		if p = strings.TrimSpace(p); p != "" {
			roots = append(roots, p)
		}
	}
	return roots
}

// cocoonResolveProjectDir —— `open <dir>` 的路径面：abs → realpath（解析链接）→ 在允许根内。
//
// 返回的 detail 是**机器面**的错因（`path_missing` / `path_not_dir` / `path_outside_root`）；
// 人面判词在调用方印（两处不同形，同一条判据）。
func cocoonResolveProjectDir(raw string) (string, string, error) {
	abs, err := filepath.Abs(raw)
	if err != nil {
		return "", "path_missing", err
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", "path_missing", fmt.Errorf("目录不存在或链接解不动：%s（%v）", raw, err)
	}
	fi, err := os.Stat(real)
	if err != nil {
		return "", "path_missing", err
	}
	if !fi.IsDir() {
		return "", "path_not_dir", fmt.Errorf("%s 不是目录（茧目录要含 %s）", raw, cocoon.SpecFileName)
	}
	for _, root := range cocoonAllowedRoots() {
		rr, rerr := filepath.EvalSymlinks(root)
		if rerr != nil {
			rr = filepath.Clean(root) // 根还不存在（如缺省状态目录）⇒ 按词法比
		}
		if pathWithinDir(rr, real) {
			return real, "", nil
		}
	}
	return "", "path_outside_root", fmt.Errorf("%s 不在允许根内（允许根：%s）",
		real, strings.Join(cocoonAllowedRoots(), " · "))
}

// cocoonValidID —— 茧 id 的**唯一**接受面：非空、不是 `.`/`..`、不含路径分隔符与控制字符。
// 与 `cocoon.WriteTombstone` 的口径同源（墓碑文件名靠 id 拼，带分隔符就能写出根外）。
func cocoonValidID(id string) bool {
	if id == "" || id == "." || id == ".." {
		return false
	}
	if strings.ContainsAny(id, `/\`) {
		return false
	}
	for _, r := range id {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

// cocoonMetaDir —— 茧目录：`<根>/<id>`。
func cocoonMetaDir(id string) string { return filepath.Join(cocoonRoot(), id) }

// cocoonReadMeta —— **只读**读 meta：不建锁件、不建目录（`cocoon.Load` 会 O_CREATE 锁件 ⇒
// 在「目录里有人手放了 meta、还没锁件」的现场上，`ls`/`inspect` 会凭空多写一个文件）。
//
// 解析口径与 `meta.go` 的 `loadLocked` 同源（同一个 `Meta` 结构 + `Validate`）；锁只在写面用。
func cocoonReadMeta(path string) (*cocoon.Meta, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", cocoon.ErrNotFound, path)
		}
		return nil, err
	}
	var m cocoon.Meta
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("%w: 解析 %s 失败: %v", cocoon.ErrInvalidMeta, path, err)
	}
	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &m, nil
}

// ── 后端 / 令牌（本版 = 假后端 + 进程内令牌管理器）────────────────────────

var (
	cocoonFakeOnce sync.Once
	cocoonFake     cocoon.Backend

	cocoonNsjailOnce sync.Once
	cocoonNsjail     cocoon.Backend

	cocoonTokensOnce sync.Once
	cocoonTokens     *cocoon.TokenManager
)

// cocoonBackendName —— 本次**实际**要用的后端名（缺省 fake）。
//
//	空 / `fake` ⇒ fake（保留既有行为，老脚本不受影响）
//	`auto`      ⇒ linux 用 nsjail、其余回 fake（**换档只允许发生在这一档**）
//	`nsjail`    ⇒ nsjail（非 linux 由 cocoonBackendByName 拒执：✗ 不在这儿悄悄改回 fake）
//	其余        ⇒ 原样返回，由 cocoonBackendByName 报「未知后端」
func cocoonBackendName() string {
	switch v := strings.TrimSpace(os.Getenv(cocoonBackendEnv)); v {
	case "":
		return cocoonBackendFake
	case cocoonBackendAuto:
		if cocoon.HostPlatform() == cocoon.PlatformLinux {
			return cocoonBackendNsjail
		}
		return cocoonBackendFake
	default:
		return v
	}
}

// cocoonBackendByName —— 名字 → 后端（**唯一**的造后端点；进程内单例）。
//
// `nsjail` 在非 linux 上返回**包着 `ErrBackendUnsupported` 的错**：显式要真隔离却拿到假
// 后端，等于把「没有隔离」冒充成「有隔离」—— 那比拒执危险得多。
func cocoonBackendByName(name string) (cocoon.Backend, error) {
	switch name {
	case "", cocoonBackendFake:
		cocoonFakeOnce.Do(func() { cocoonFake = cocoon.NewFakeBackend() })
		return cocoonFake, nil
	case cocoonBackendNsjail:
		if cocoon.HostPlatform() != cocoon.PlatformLinux {
			return nil, fmt.Errorf("%w: 后端 %q 只在 %s 上可用（本平台 %s，未接）；"+
				"要脚手架请显式写 %s=%s，要自动选档请写 %s",
				cocoon.ErrBackendUnsupported, cocoonBackendNsjail, cocoon.PlatformLinux,
				cocoon.HostPlatform(), cocoonBackendEnv, cocoonBackendFake, cocoonBackendAuto)
		}
		cocoonNsjailOnce.Do(func() {
			nb := cocoon.NewNsjailBackend()
			// ★ 共享宿主 netns：命令面承诺的是「茧内 127.0.0.1:<port> 就是宿主那个端口」
			//   （Expose=false 的全部依据），也是「AI 端点走 127.0.0.1」的前提 —— nsjail 缺省
			//   会建**新** netns，那样茧内回环是它自己的，宿主与 broker 都够不着。
			//   ★ 代价如实登记：茧因此**能出网**（运行期 net=none 未接，见文件头）。
			nb.HostNetwork = true
			cocoonNsjail = nb
		})
		return cocoonNsjail, nil
	}
	return nil, fmt.Errorf("未知后端 %q（%s 只接受 %s / %s / %s）",
		name, cocoonBackendEnv, cocoonBackendFake, cocoonBackendNsjail, cocoonBackendAuto)
}

// cocoonBackendFactory —— 造后端的唯一出口（**测试要能换掉它**：演「不映射端口」这类后端；
// 生产路径永远是 cocoonBackendByName）。
var cocoonBackendFactory = cocoonBackendByName

// cocoonBackendCaps —— 后端自报的能力面（inspect / open 的端口决策依据）。
//
// ★ 先问后端（`CapabilityReporter`），后端不报才退回**平台表**；两处都拿不到就是错
// —— ✗ 不许给一张「全 false 的表」冒充「后端在，只是弱」。
func cocoonBackendCaps(be cocoon.Backend) (cocoon.Capabilities, error) {
	if r, ok := be.(cocoon.CapabilityReporter); ok {
		return r.Capabilities()
	}
	if c, ok := cocoon.PlatformCapabilities(cocoon.HostPlatform()); ok {
		return c, nil
	}
	return cocoon.Capabilities{}, fmt.Errorf("平台 %q 没有能力声明（后端也没自报）", cocoon.HostPlatform())
}

// cocoonWireCocoonDir —— 把茧目录交给后端（`CocoonDirSetter`；后端不需要落盘就什么都不做）。
//
// ★ 为什么要过一道接口而不是直接写 `NsjailBackend.BaseDir`：
//
//	· 命令面只认接口（与 CapabilityReporter / HandleReaper 同族），假后端与测试桩按需实现；
//	· 后端自己校验「目录存在且是目录」⇒「先建目录再传」这条时序由后端把住，✗ 不是注释约定。
//
// ★ dir 必须是**茧目录本身**（真源根下的 `<id>/`）：后端的实例根是 `<dir>/<后端签发的 id>`，
//
//	overlay 的 upper/work/merged 与茧内 /tmp 全落在这棵下面 ⇒ `close --purge` 删茧目录时
//	一次收干净，也不会碰到兄弟茧。
//
// ★ `open` 与 `close` **都**要设：close 是另一个进程，凭 meta 里的句柄收尾时要靠 BaseDir
//
//	算出茧根（`NsjailBackend.reapRoot` 证明「这个目录在我 BaseDir 下一层」才肯删）。
func cocoonWireCocoonDir(be cocoon.Backend, dir string) error {
	s, ok := be.(cocoon.CocoonDirSetter)
	if !ok {
		return nil // 该后端不落盘（假后端一类）：没有茧目录这回事
	}
	return s.SetCocoonDir(dir)
}

// cocoonTokenMgr —— 令牌管理器单例（本版进程内；`close` 吊销的就是同一张表里的那一枚）。
func cocoonTokenMgr() *cocoon.TokenManager {
	cocoonTokensOnce.Do(func() { cocoonTokens = cocoon.NewTokenManager() })
	return cocoonTokens
}

// ── 小工具 ──────────────────────────────────────────────────────────────

// cocoonPortsText —— 端口映射的稳定文本：`茧内→宿主`（按茧内端口升序，`;` 连接）。
func cocoonPortsText(ports map[int]int) string {
	if len(ports) == 0 {
		return cocoonDash
	}
	inner := make([]int, 0, len(ports))
	for k := range ports {
		inner = append(inner, k)
	}
	sort.Ints(inner)
	parts := make([]string, 0, len(inner))
	for _, k := range inner {
		parts = append(parts, fmt.Sprintf("%d→%d", k, ports[k]))
	}
	return strings.Join(parts, ";")
}

// cocoonOrDash —— 空串 → `—`（三值纪律：没有值就明说「没有」，✗ 印空串）。
func cocoonOrDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return cocoonDash
	}
	return s
}

// cocoonStamp —— 时刻的稳定文本（UTC · RFC3339）；零值 → `—`。
func cocoonStamp(t time.Time) string {
	if t.IsZero() {
		return cocoonDash
	}
	return t.UTC().Format(time.RFC3339)
}

// cocoonMetaRow —— meta 的公共机器面行（ls / inspect / close 共用同一份取值口径）。
func cocoonMetaRow(m *cocoon.Meta, dir string) map[string]string {
	row := map[string]string{
		"id":         m.ID,
		"phase":      string(m.Phase),
		"pid":        strconv.Itoa(m.PID),
		"ports":      cocoonPortsText(m.Ports),
		"egg":        cocoonOrDash(m.Egg),
		"owner":      cocoonOrDash(m.Owner),
		"created_at": cocoonStamp(m.CreatedAt),
		"dir":        dir,
	}
	if m.StartTime != nil {
		row["start_time"] = cocoonStamp(*m.StartTime)
	}
	return row
}

// ── 契约哈希锁（判据 9 · v2 §E）───────────────────────────────────────────

// cocoonSpecLockLivePhase —— 哪些 phase 的茧还「压着」契约锁。
//
// ★ 口径：锁只约束**未到终态**的茧（created/starting/running/failed/stopping）。
// 终态（gone/deleting）的茧已经不再代表这份契约的任何执行体 —— 若照样拿它的锁拦人，
// 「关掉一个茧 ⇒ 改契约 ⇒ 重开」这条最正常的路就被永久封死（关过一次之后再也起不来）。
// 反过来，只要还有一个活茧在跑，拿改过的契约再起一个就把「盘上两个茧声称同一份契约」变成谎。
func cocoonSpecLockLivePhase(p cocoon.Phase) bool {
	switch p {
	case cocoon.PhaseCreated, cocoon.PhaseStarting, cocoon.PhaseRunning,
		cocoon.PhaseFailed, cocoon.PhaseStopping:
		return true
	}
	return false
}

// cocoonSpecDriftCheck —— `open` 的判据 9 面：同一契约目录**已有活茧**时，现读的 `cocoon.toml`
// 必须与它 meta 里记的 `SpecSHA` 逐字节一致；不一致 ⇒ 拒（返回的错里带期望/实得各前 8 位）。
//
// ★ **只读**：扫的是各茧的 `meta.json`（走 `cocoonReadMeta` —— 不建锁件），不比 `cocoon.Load`
// 那条会 O_CREATE `.lock` 的路。拒绝发生时状态根里一个条目都不许多（测试按整树对拍）。
// ★ 孤儿/坏账（有目录无 meta / meta 读不出）**不在这里判**：那是 `reconcile` 的话。这里跳过，
// 否则一个坏账就能把 open 永久拦住，而人还得先跑对账才能起茧。
func cocoonSpecDriftCheck(specPath, dir string) error {
	root := cocoonRoot()
	entries, err := os.ReadDir(root)
	if err != nil {
		// 状态根还没有（第一次 open）/ 读不了 ⇒ 没有可比对的锁。读不了的账由后面写 meta 时如实报，
		// 不在这里伪装成「漂移」（把权限问题说成契约问题，等于让人的修法跑偏）。
		return nil
	}
	ids := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() && cocoonValidID(e.Name()) {
			ids = append(ids, e.Name())
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		m, rerr := cocoonReadMeta(cocoon.MetaPath(filepath.Join(root, id)))
		if rerr != nil || m.SpecSHA == "" || m.SpecDir == "" || m.SpecDir != dir {
			continue
		}
		if !cocoonSpecLockLivePhase(m.Phase) {
			continue
		}
		if cerr := cocoon.CheckSpec(specPath, m.SpecSHA); cerr != nil {
			return fmt.Errorf("茧 %s 锁的契约与现读不符：%w", id, cerr)
		}
	}
	return nil
}

// cocoonSpecLockRow —— `inspect` 的契约锁三格（只读：`ProbeSpec` 不写盘）。
//
// 返回 state 闭集四值：`ok`（一致）· `drift`（漂移）· `unlocked`（meta 里没有锁值 —— 老茧/
// 脚本写入，✗ 不当成一致）· `unreadable`（契约读不到，判不了）。
type cocoonSpecLockView struct {
	State   string
	Locked  string
	Current string
	Detail  string
}

func cocoonSpecLockOf(m *cocoon.Meta) cocoonSpecLockView {
	if m.SpecSHA == "" || m.SpecDir == "" {
		return cocoonSpecLockView{State: "unlocked"}
	}
	st := cocoon.ProbeSpec(filepath.Join(m.SpecDir, cocoon.SpecFileName), m.SpecSHA)
	v := cocoonSpecLockView{Locked: st.Locked, Current: st.Current}
	switch {
	case st.Err == nil:
		v.State = "ok"
	case errors.Is(st.Err, cocoon.ErrSpecDrift):
		v.State, v.Detail = "drift", st.Err.Error()
	case errors.Is(st.Err, cocoon.ErrSpecUnreadable):
		v.State, v.Detail = "unreadable", st.Err.Error()
	default:
		// 没有锁值 / 未认出的档一律按「判不了」报（✗ 不猜成一致）。
		v.State, v.Detail = "unlocked", st.Err.Error()
	}
	return v
}

// cocoonSpecOfMeta — **只读**回读 meta 记下的那份契约（`open` 的注入形状就是按它算的）。
//
// 返回 (spec, why)：读不到 / 解析不过 ⇒ spec=nil 且 why 是原因。调用方按三值纪律处理
// （有形状 / 没开 / 取不到数），✗ 不许把"取不到"伪装成"没开"。
//
// ★ 为什么不把形状写进 meta：meta 是要给人看、给脚本读的盘上件，而"注入面"的真源只有
// 契约一处。写两份就会有两种说法（契约改了、meta 还留着旧形状），而克隆出来的那份
// 特别容易被读成"这就是茧内真拿到的"。
func cocoonSpecOfMeta(m *cocoon.Meta) (*cocoon.Spec, string) {
	dir := strings.TrimSpace(m.SpecDir)
	if dir == "" {
		return nil, "meta 没记契约目录（老茧 / 脚本写的 meta）"
	}
	p := filepath.Join(dir, cocoon.SpecFileName)
	raw, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Sprintf("契约读不到（%s）", p)
	}
	spec, perr := cocoon.ParseSpecFile(p, raw)
	if perr != nil {
		return nil, fmt.Sprintf("契约解析不过（%v）", perr)
	}
	return spec, ""
}

// cocoonAllocID —— 分配茧 id：`<契约名>-<8 位十六进制>`，并确保 `<根>/<id>` 与墓碑都不在盘上。
func cocoonAllocID(name string) (string, error) {
	base := strings.ToLower(strings.TrimSpace(name))
	if base == "" {
		base = "cocoon"
	}
	var b strings.Builder
	for _, r := range base {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	base = strings.Trim(b.String(), "-")
	if base == "" {
		base = "cocoon"
	}
	for i := 0; i < 8; i++ {
		buf := make([]byte, cocoonIDHexBytes)
		if _, err := rand.Read(buf); err != nil {
			return "", fmt.Errorf("茧 id 随机源不可用: %w", err)
		}
		id := base + "-" + hex.EncodeToString(buf)
		if _, err := os.Stat(cocoonMetaDir(id)); err == nil {
			continue
		}
		if _, err := os.Stat(cocoon.FlatMetaPath(cocoonRoot(), id)); err == nil {
			continue
		}
		return id, nil
	}
	return "", errors.New("茧 id 分配失败（连续 8 次撞名）")
}

// cocoonOpenUsage — `open` 的用法面拒执（缺参/多参同一句话，✗ 各写一套）。
// ★ 必须落 `kind=usage`：dispatch 的失败包封只给**非 usage** 的失败补 stdout（K2 口径 =
// 用法错 stdout 零字节）—— 不落 kind 时 `--json` 会意外补出一个 409 字节的 error 包封。
func cocoonOpenUsage(inv *invocation, stderr io.Writer, why string) int {
	inv.setErr("usage", "open_needs_dir", why)
	fmt.Fprintf(stderr, "%s: %s\n", progName, why)
	fmt.Fprintf(stderr, "用法：%s cocoon open <dir> [--dry-run] [--json <字段>]\n", progName)
	return exitUsage
}

// ── AI 注入面（T4）─────────────────────────────────────────────────────

// cocoonAIUpstream — AI base 的三级真源：契约 `[茧.ai].base` ＞ `ZERG_AI_BASE` ＞ 内置回环。
//
// ★ 为什么"缺省取宿主真源"而不是把内置值抄进契约：base 指向的是**宿主上的 broker**，
// 它的端口由 broker bind 后回写 ⇒ 契约里写死端口等于让契约去猜一个运行期才存在的值。
// ★ 为什么 env 这一级不是 `ZERG_AI_UPSTREAM`：那个名字是 broker 的**真上游**（`cocoon.EnvAIUpstream`，
// 缺省 9400）。混用会把"宿主让 broker 指向模型"读成"让茧直连模型" —— 茧内就能绕过 broker
// 换值那一跳，占位符形同虚设。两个 env 各管一段，是本件的一条判据（测试逐条打）。
//
// ★ 第三级（内置回环）**不在本件拼**：它走 `cocoon.BrokerAIBase()`，与 broker 的监听地址
// 同一个真源 ⇒ `ZERG_BROKER_ADDR` 一处旋钮同时改两处（T4c）。
// 旋钮非法（不是 host:port / 不是回环）⇒ **上抛错误**、由调用方拒执：编一个「看起来能用」
// 的地址会让茧指着一扇没人开的门（那正是 T4 的病），比当场退 1 更坏。
func cocoonAIUpstream(ai cocoon.AISpec) (string, error) {
	if b := strings.TrimSpace(ai.Base); b != "" {
		return b, nil
	}
	if v := strings.TrimSpace(os.Getenv(cocoonAIBaseEnv)); v != "" {
		return v, nil
	}
	base, _, err := cocoon.BrokerAIBase()
	if err != nil {
		return "", fmt.Errorf("茧内缺省 base 算不出来（宿主 broker 地址旋钮 %s／%s 非法）：%w",
			cocoon.EnvBrokerAddr, cocoon.EnvBrokerHTTPAddr, err)
	}
	return base, nil
}

// cocoonAIInject — 按契约算出「要注入给茧内的三件东西」＋需要的那条挂载项。
//
//	返回零值 plan ⇒ 契约没开 AI（安全侧：茧内拿不到任何 AI env）。
//	mounts        ⇒ 换了根时那条 **ro** 绑定；没换根 ⇒ nil（宿主路径在茧内本来就可见）。
//
// ★ 本函数**不写盘、不碰网**：`--dry-run` 也要能算同一份形状，而干跑必须零副作用。
//
//	写占位符件那一步由调用方（真跑档）单独做 —— 两件事分开，干跑就不可能偷偷落了件。
func cocoonAIInject(instID, tokenHost string, spec *cocoon.Spec) (cocoon.AIPlan, []cocoon.OrchestrateMount, error) {
	if spec == nil || !spec.Cocoon.AI.Enabled {
		return cocoon.AIPlan{}, nil, nil
	}
	base, berr := cocoonAIUpstream(spec.Cocoon.AI)
	if berr != nil {
		return cocoon.AIPlan{}, nil, berr
	}
	plan, err := cocoon.BuildAIEnvChecked(instID, spec.Cocoon.AI.ModelOrDefault(), base)
	if err != nil {
		return cocoon.AIPlan{}, nil, err
	}
	// ★「换不换根」只问一个口（cocoon.CocoonChroots），✗ 不在这里自己看 rootfs 串：
	//   两处各判一次，迟早会有一处把 `rootfs = "/"`（等价于不换根）当成换根。
	chrooted := cocoon.CocoonChroots(spec)
	plan, err = plan.WithTokenFile(cocoon.AITokenInCocoonPath(chrooted, tokenHost))
	if err != nil {
		return cocoon.AIPlan{}, nil, err
	}
	var mounts []cocoon.OrchestrateMount
	if mnt, ok := cocoon.AITokenMount(chrooted, tokenHost); ok {
		mounts = append(mounts, mnt)
	}
	return plan, mounts, nil
}

// cocoonAIText — 人面那一行（真跑/干跑共用同一句话，✗ 不许两处各说一套）。
func cocoonAIText(plan cocoon.AIPlan) string {
	return fmt.Sprintf("base=%s · model=%s · 令牌件=%s（内容恒为占位符 %s · 真值不在茧内 · "+
		"茧内直连 %s 的 /health 与 /v1/chat/completions，由宿主 broker 换真令牌再拨上游；"+
		"前置条件：宿主先 `%s cocoon broker start`，否则茧内 curl 得到连接失败）",
		plan.Base, plan.Model, plan.TokenFile, cocoon.AITokenPlaceholder, plan.Base, progName)
}

// ── open ────────────────────────────────────────────────────────────────

// cmdCocoonOpen —— `zerg cocoon open <dir> [--dry-run]`。
//
// 真跑的顺序（每一步都先落盘再动，崩在哪一步盘上都看得见）：
//
//	读 <dir>/cocoon.toml → 校验 → 分配 id → 写 meta(created) → 落 starting
//	→ Provision → 按 Spec.Order 起服务 → Expose 端口 → 写 meta(running, pid, ports) → 输出 id 与端口
//
// `--dry-run`：只出计划件，**零副作用**（不建目录、不写 meta、不 Provision、不起进程、不占端口）。
func cmdCocoonOpen(inv *invocation, stdout, stderr io.Writer) int {
	if len(inv.args) != 1 {
		return cocoonOpenUsage(inv, stderr, "`cocoon open` 要且只要一个茧目录")
	}
	dir, detail, err := cocoonResolveProjectDir(inv.args[0])
	if err != nil {
		inv.setErr("usage", detail, err.Error())
		fmt.Fprintf(stderr, "%s: %v ⇒ 退码 2（路径入参先 realpath 再校验允许根）\n", progName, err)
		return exitUsage
	}
	specPath := filepath.Join(dir, cocoon.SpecFileName)
	// ★ 契约字节**只读一次**：现读 → 解析 → 算锁值，三步用同一份字节。
	//   分两次读会在两次之间留一个窗口（`LoadSpec` 读一份、`LockSpec` 读另一份 ⇒ 锁上的
	//   哈希可能是另一份契约的）。`ParseSpecFile` 与 `LoadSpec` 同一条解析路（含来源名）。
	rawSpec, rerr := os.ReadFile(specPath)
	if rerr != nil {
		inv.setErr("failed", "spec_missing", rerr.Error())
		fmt.Fprintf(stderr, "%s: 读不到契约 %s：%v ⇒ 退码 1\n", progName, specPath, rerr)
		return exitFail
	}
	spec, err := cocoon.ParseSpecFile(specPath, rawSpec)
	if err != nil {
		var se *cocoon.SpecError
		if errors.As(err, &se) {
			inv.setErr("usage", "spec_invalid", err.Error())
			fmt.Fprintf(stderr, "%s: 契约校验未过：%v ⇒ 退码 2（一个字节没写）\n", progName, err)
			return exitUsage
		}
		inv.setErr("failed", "spec_missing", err.Error())
		fmt.Fprintf(stderr, "%s: 读不到契约 %s：%v ⇒ 退码 1\n", progName, specPath, err)
		return exitFail
	}
	// 判据 9（v2 §E）：契约哈希锁 —— 同一目录**已有活茧**时，现读字节必须与它 meta 里记的
	// `SpecSHA` 逐字节一致；不一致 ⇒ 拒（判在分配 id / 写 meta 之前 ⇒ 零副作用）。
	// `--dry-run` 也照样判：干跑若放行、真跑才拒，那计划件就是在骗人。
	if derr := cocoonSpecDriftCheck(specPath, dir); derr != nil {
		inv.setErr("failed", "spec_drift", derr.Error())
		fmt.Fprintf(stderr, "%s: 契约漂移 ⇒ 退码 1（未写 meta / 未 Provision / 未起服务）：%v\n", progName, derr)
		fmt.Fprintf(stderr, "%s: 要么把 %s 改回锁定的那一份，要么先关掉同目录的活茧再改\n", progName, specPath)
		return exitFail
	}
	specSHA, serr := cocoon.LockSpec(rawSpec)
	if serr != nil {
		inv.setErr("usage", "spec_invalid", serr.Error())
		fmt.Fprintf(stderr, "%s: 契约算不出锁值：%v ⇒ 退码 2（一个字节没写）\n", progName, serr)
		return exitUsage
	}
	backendName := cocoonBackendName()
	// ★ 后端选档在**任何副作用之前**：名字不认识 / 平台不支持 ⇒ 零写入拒执
	//   （`--dry-run` 同拒 —— 干跑放行、真跑才拒，那计划件就是在骗人）。
	be, err := cocoonBackendFactory(backendName)
	if err != nil {
		inv.setErr("failed", "backend_unavailable", err.Error())
		fmt.Fprintf(stderr, "%s: %v ⇒ 退码 1（未写 meta / 未 Provision / 未起服务）\n", progName, err)
		return exitFail
	}
	caps, cerr := cocoonBackendCaps(be)
	if cerr != nil {
		inv.setErr("failed", "backend_unavailable", cerr.Error())
		fmt.Fprintf(stderr, "%s: 后端 %q 的能力面不可用：%v ⇒ 退码 1（未写 meta / 未起服务）\n",
			progName, backendName, cerr)
		return exitFail
	}

	if inv.dryRun {
		return cocoonOpenPlan(inv, stdout, stderr, dir, spec, backendName, caps)
	}

	id, err := cocoonAllocID(spec.Cocoon.Name)
	if err != nil {
		inv.setErr("failed", "id_alloc_failed", err.Error())
		fmt.Fprintf(stderr, "%s: %v ⇒ 退码 1\n", progName, err)
		return exitFail
	}
	cdir := cocoonMetaDir(id)
	metaPath := cocoon.MetaPath(cdir)
	owner := strings.TrimSpace(os.Getenv("USER"))
	if owner == "" {
		owner = "cli"
	}
	m := cocoon.New(id, "", owner)
	// 判据 9 的锁值随 meta 一起落盘（**先写 meta 再动**这条不变：锁与 phase 同一次写入）。
	m.SpecSHA = specSHA
	m.SpecDir = dir
	// 后端名先落盘：崩在中途时，盘上至少知道「这是哪个后端的茧」，close 不必按平台猜。
	m.Backend = backendName
	if err := cocoon.Save(metaPath, m); err != nil {
		inv.setErr("failed", "meta_write_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 写 meta 失败（%s）：%v ⇒ 退码 1\n", progName, metaPath, err)
		return exitFail
	}
	if _, err := cocoon.UpdatePhase(metaPath, cocoon.PhaseStarting); err != nil {
		inv.setErr("failed", "meta_write_failed", err.Error())
		fmt.Fprintf(stderr, "%s: meta 落 starting 失败：%v ⇒ 退码 1\n", progName, err)
		return exitFail
	}

	// ★ 茧目录先建、再交给后端（`SetCocoonDir` 只校验不建盘 ⇒ 顺序反了会当场红）。
	//   不靠 `cocoon.Save` 的 MkdirAll 副作用：那是 meta 落盘的内部实现，时序在这里是判据
	//   （rootfs 的 upper/work/merged 与实例根都要落进这个目录）。
	if mkerr := os.MkdirAll(cdir, 0o700); mkerr != nil {
		inv.setErr("failed", "cocoon_dir_failed", mkerr.Error())
		fmt.Fprintf(stderr, "%s: 建茧目录 %s 失败：%v ⇒ 退码 1（未 Provision / 未起服务）\n",
			progName, cdir, mkerr)
		return exitFail
	}
	// rootfs 的落点：后端据此把 overlay 的 upper/work/merged 与实例根放进茧目录。
	if werr := cocoonWireCocoonDir(be, cdir); werr != nil {
		inv.setErr("failed", "backend_unavailable", werr.Error())
		fmt.Fprintf(stderr, "%s: 后端不接受茧目录 %s：%v ⇒ 退码 1（未 Provision / 未起服务）\n",
			progName, cdir, werr)
		return exitFail
	}

	// ── AI 注入面（T4）─────────────────────────────────────────────────────
	//
	// ★ 真跑档才落件（干跑在上面已经 return 了）—— 干跑只出计划，✗ 不写盘。
	// ★ 落的是**占位符**（`zerg-broker`），真令牌既不写盘也不入 env：真值只在宿主 broker。
	var (
		aiPlan   cocoon.AIPlan
		aiMounts []cocoon.OrchestrateMount
	)
	if spec.Cocoon.AI.Enabled {
		tokenHost, werr := cocoon.WriteAITokenPlaceholder(cdir)
		if werr != nil {
			if _, uerr := cocoon.UpdatePhase(metaPath, cocoon.PhaseFailed); uerr != nil {
				fmt.Fprintf(stderr, "%s: 另：meta 落 failed 也失败：%v\n", progName, uerr)
			}
			inv.setErr("failed", "ai_token_file_failed", werr.Error())
			fmt.Fprintf(stderr, "%s: 写 AI 占位符件失败（已停在这一步 · meta 落 failed · 未 Provision）：%v ⇒ 退码 1\n",
				progName, werr)
			return exitFail
		}
		p, ms, perr := cocoonAIInject(id, tokenHost, spec)
		if perr != nil {
			if _, uerr := cocoon.UpdatePhase(metaPath, cocoon.PhaseFailed); uerr != nil {
				fmt.Fprintf(stderr, "%s: 另：meta 落 failed 也失败：%v\n", progName, uerr)
			}
			inv.setErr("failed", "ai_inject_failed", perr.Error())
			fmt.Fprintf(stderr, "%s: 算不出 AI 注入形状（已停在这一步 · meta 落 failed · 未起服务）：%v ⇒ 退码 1\n",
				progName, perr)
			return exitFail
		}
		aiPlan, aiMounts = p, ms
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// 服务的 stdout/stderr 落 <茧目录>/logs/<服务>.log：起服务的是**本进程**，而本进程会退出
	// —— 管道读端随之消失，长驻服务下一次写就会撞 EPIPE（见 cocoon.ExecOpts.LogPath）。
	//
	// ★ Detach=true 是这一单的核心：open 是一次性进程，阻塞档里服务被它一起带走
	//   （T3b 实测三次真后端 open：pid:0、无 logs/、无 nsjail 进程）。
	logDir := filepath.Join(cdir, "logs")
	opts := cocoon.OrchestrateOptions{LogDir: logDir, Detach: true}
	// AI 注入面：三个 env（base/model/令牌件路径）＋换了根时那条 ro 绑定。
	// ★ 两者都只在 `[茧.ai] enabled = true` 时非空（零值即"没有 AI 能力"）。
	if aiPlan.Valid() {
		opts.AIEnv = aiPlan.Env
		opts.Mounts = aiMounts
	}
	if !caps.Expose {
		// 后端不映射端口 ⇒ readiness 探测直连回环（茧与调用方同一网络栈，茧内端口就是宿主端口）。
		// 走 Expose 会拿到 ErrBackendUnsupported —— 那是「这条链路不存在」，不是「端口不通」。
		opts.ProbeAddr = func(_ cocoon.ServiceSpec, rd cocoon.Readiness) (string, error) {
			return fmt.Sprintf("%s:%d", cocoon.DefaultBind, rd.Port), nil
		}
	}
	r, err := cocoon.StartWith(ctx, be, spec, opts)
	if err != nil {
		// Start 失败路径自己已经把已起的服务逆序停掉、把实例 Destroy 掉（见 orchestrate.go）
		// ⇒ 这里只负责把盘上的结论改成 failed（**不撒谎成 running**）。
		if _, uerr := cocoon.UpdatePhase(metaPath, cocoon.PhaseFailed); uerr != nil {
			fmt.Fprintf(stderr, "%s: 另：meta 落 failed 也失败：%v\n", progName, uerr)
		}
		inv.setErr("failed", "orchestrate_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 起服务失败（已逆序回滚 · meta 落 failed）：%v ⇒ 退码 1\n", progName, err)
		return exitFail
	}
	inst := r.Instance()
	ports := map[int]int{}
	switch {
	case caps.Expose:
		for _, p := range spec.Ports {
			hp, perr := be.Expose(ctx, inst, p.Inner, p.Bind)
			if perr != nil {
				_ = cocoon.Stop(ctx, r)
				_ = be.Destroy(ctx, inst)
				if _, uerr := cocoon.UpdatePhase(metaPath, cocoon.PhaseFailed); uerr != nil {
					fmt.Fprintf(stderr, "%s: 另：meta 落 failed 也失败：%v\n", progName, uerr)
				}
				inv.setErr("failed", "expose_failed", perr.Error())
				fmt.Fprintf(stderr, "%s: 露端口 %d 失败（已停服务 · meta 落 failed）：%v ⇒ 退码 1\n", progName, p.Inner, perr)
				return exitFail
			}
			ports[p.Inner] = hp
		}
	case len(spec.Ports) > 0:
		// ★ 跳过要**说出来**：契约写了 3 条端口却一条没映射，静默走下只会让人以为端口在。
		fmt.Fprintf(stderr, "%s: 后端 %s 不映射端口（%s）⇒ 契约的 %d 条 [[端口]] 跳过，直连 %s:<茧内端口>\n",
			progName, backendName, caps.ExposeMechanism, len(spec.Ports), cocoon.DefaultBind)
	}

	// ★ 后端**签发的**句柄落盘（实例 id + 活 pid 表）：`close` 是另一个进程，内存记账那时已经没了。
	//   这是「跨进程收干净」的唯一依据 —— 不落它，close 只能报成功却什么都没收。
	var pids []int
	if pr, ok := be.(cocoon.PIDReporter); ok {
		// ★ 等一小窗再读：Spawn 只是把后端 Exec 丢进 goroutine，pid 由**子进程那边**写进后端记账
		//   ⇒ 立刻读可能读到空（实测：声明了 readiness 的服务因为多探了几轮而躲过这个竞态，
		//   没声明 readiness 的服务则被误判成「后端没给 pid」）。这不是「放宽判据」：
		//   窗口过后仍没有 pid，照样按失败报。
		pids = cocoonAwaitPids(pr, inst)
	}

	// ★★ 后台档**验活**（本单的核心判据）：Start 返回只代表「进程签出了」，不代表「服务在跑」。
	//   T3b 的假绿正是死在这里：open 报成功、盘上 pid:0、茧里其实什么都没有。
	//   判据只有一条：后端自报的 pid，此刻**真在**（连续一窗活着）。退出即失败。
	if why := cocoonVerifyDetachedAlive(be, pids); len(why) > 0 {
		// 起过的东西逆序停掉 + Destroy（与露端口失败同一条回滚路），meta 落 failed、绝不写 running。
		_ = cocoon.Stop(ctx, r)
		_ = be.Destroy(ctx, inst)
		if _, uerr := cocoon.UpdatePhase(metaPath, cocoon.PhaseFailed); uerr != nil {
			fmt.Fprintf(stderr, "%s: 另：meta 落 failed 也失败：%v\n", progName, uerr)
		}
		inv.setErr("failed", "service_not_alive", strings.Join(why, " · "))
		fmt.Fprintf(stderr, "%s: 后台档验活未过（已停服务 · meta 落 failed，✗ 不写 running）：%s ⇒ 退码 1\n",
			progName, strings.Join(why, " · "))
		for _, line := range why {
			fmt.Fprintf(stderr, "  · %s\n", line)
		}
		fmt.Fprintf(stderr, "  日志：%s（茧内服务自己的输出都在这里，没起来时先看它）\n", logDir)
		return exitFail
	}

	m2, err := cocoon.Load(metaPath)
	if err != nil {
		inv.setErr("failed", "meta_read_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 回读 meta 失败：%v ⇒ 退码 1\n", progName, err)
		return exitFail
	}
	now := cocoon.Now()
	m2.PID = inst.PID
	if m2.PID == 0 && len(pids) > 0 {
		m2.PID = pids[0]
	}
	m2.Ports = ports
	m2.StartTime = &now
	m2.Backend = backendName
	m2.BackendInstance = inst.ID
	m2.BackendPIDs = pids
	if err := m2.SetPhase(cocoon.PhaseRunning); err != nil {
		inv.setErr("failed", "meta_write_failed", err.Error())
		fmt.Fprintf(stderr, "%s: meta 落 running 失败：%v ⇒ 退码 1\n", progName, err)
		return exitFail
	}
	if err := cocoon.Save(metaPath, m2); err != nil {
		inv.setErr("failed", "meta_write_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 写 meta(running) 失败：%v ⇒ 退码 1\n", progName, err)
		return exitFail
	}
	tok, terr := cocoonTokenMgr().IssueToken(id)
	tokenText := cocoonDash
	if terr == nil {
		tokenText = tok.Redacted()
	}

	row := cocoonMetaRow(m2, cdir)
	row["pid"] = strconv.Itoa(m2.PID)
	row["backend"] = backendName
	row["token"] = tokenText
	row["logs"] = cocoonLogsCell(logDir)
	if inv.jsonGiven {
		return listCmd(inv, stdout, stderr, cocoonLsDisplay, []map[string]string{row})
	}
	fmt.Fprintf(stdout, "茧 id      : %s\n", m2.ID)
	fmt.Fprintf(stdout, "契约       : %s（name=%s · image=%s · digest=%s · arch=%s）\n",
		specPath, spec.Cocoon.Name, spec.Cocoon.Image, cocoonOrDash(spec.Cocoon.Digest), cocoonOrDash(spec.Cocoon.Arch))
	fmt.Fprintf(stdout, "契约锁     : sha256 %s…（inspect 与本目录后续 open 都按它判漂移 · 全文在 meta.json 的 spec_sha）\n",
		cocoon.SpecShort(specSHA))
	fmt.Fprintf(stdout, "后端       : %s · %s · 平台 %s%s\n", backendName, cocoonOrDash(caps.Isolation), caps.Platform,
		map[bool]string{true: "（脚手架：不起进程 / 无隔离）", false: ""}[backendName == cocoonBackendFake])
	fmt.Fprintf(stdout, "相位       : %s\n", m2.Phase)
	pidNote := ""
	if m2.PID == 0 {
		pidNote = "（本后端不起进程或还没起）"
	} else if len(m2.BackendPIDs) > 0 {
		pidNote = fmt.Sprintf("（后端签发的 exec 主进程：%v · 已落 meta.backend_pids · 已验活）", m2.BackendPIDs)
	}
	fmt.Fprintf(stdout, "pid        : %d%s\n", m2.PID, pidNote)
	fmt.Fprintf(stdout, "日志       : %s（本进程退出后服务照旧往这里写；起不来先看它）\n", cocoonLogsCell(logDir))
	if caps.Expose {
		fmt.Fprintf(stdout, "端口       : %s\n", cocoonPortsText(ports))
	} else if len(spec.Ports) > 0 {
		fmt.Fprintf(stdout, "端口       : %s（后端不映射：直连 %s:<茧内端口>；理由见 stderr）\n",
			cocoonDash, cocoon.DefaultBind)
	} else {
		fmt.Fprintf(stdout, "端口       : %s\n", cocoonPortsText(ports))
	}
	fmt.Fprintf(stdout, "服务       : %s\n", strings.Join(spec.Order, " → "))
	if aiPlan.Valid() {
		fmt.Fprintf(stdout, "AI         : %s\n", cocoonAIText(aiPlan))
	} else {
		fmt.Fprintf(stdout, "AI         : %s（契约未开 [茧.ai].enabled ⇒ 不给 AI 能力，是安全侧）\n", cocoonDash)
	}
	fmt.Fprintf(stdout, "令牌       : %s（只出脱敏形；真值进茧内 env，✗ 不进 stdout/argv/日志）\n", tokenText)
	fmt.Fprintf(stdout, "状态根     : %s\n", cdir)
	return exitOK
}

// cocoonOpenPlan —— `--dry-run` 的计划件（零副作用：本函数只读 spec，不碰盘）。
func cocoonOpenPlan(inv *invocation, stdout, stderr io.Writer, dir string, spec *cocoon.Spec,
	backendName string, caps cocoon.Capabilities) int {
	ports := make([]string, 0, len(spec.Ports))
	for _, p := range spec.Ports {
		ports = append(ports, fmt.Sprintf("%d/%s/%s", p.Inner, cocoonOrDash(p.Host), cocoonOrDash(p.Bind)))
	}
	portText := strings.Join(ports, " · ")
	if portText == "" {
		portText = cocoonDash
	}
	svcText := strings.Join(spec.Order, " → ")
	if svcText == "" {
		svcText = cocoonDash
	}
	row := map[string]string{
		"id": "", "phase": "planned", "pid": "0", "ports": portText,
		"egg": cocoonDash, "backend": backendName, "token": cocoonDash,
		"dir": filepath.Join(cocoonRoot(), "<分配于真跑>"),
	}
	if inv.jsonGiven {
		return listCmd(inv, stdout, stderr, cocoonLsDisplay, []map[string]string{row})
	}
	fmt.Fprintln(stdout, "计划件（--dry-run · 零副作用 —— 未写 meta、未 Provision、未起服务、未占端口）")
	fmt.Fprintf(stdout, "  动作    : %s cocoon open\n", progName)
	fmt.Fprintf(stdout, "  契约    : %s/%s（校验通过）\n", dir, cocoon.SpecFileName)
	fmt.Fprintf(stdout, "  茧名    : %s\n", spec.Cocoon.Name)
	fmt.Fprintf(stdout, "  镜像    : %s（digest %s · arch %s）\n",
		spec.Cocoon.Image, cocoonOrDash(spec.Cocoon.Digest), cocoonOrDash(spec.Cocoon.Arch))
	fmt.Fprintf(stdout, "  服务    : %s\n", svcText)
	fmt.Fprintf(stdout, "  端口    : %s\n", portText)
	fmt.Fprintf(stdout, "  资源    : cpu=%s mem_gb=%s disk_gb=%s\n",
		strconv.FormatFloat(spec.Resources.CPU, 'f', -1, 64),
		strconv.FormatFloat(spec.Resources.MemGB, 'f', -1, 64),
		strconv.FormatFloat(spec.Resources.DiskGB, 'f', -1, 64))
	fmt.Fprintf(stdout, "  后端    : %s · %s · 平台 %s\n", backendName, cocoonOrDash(caps.Isolation), caps.Platform)
	if !caps.Expose {
		fmt.Fprintf(stdout, "  端口面  : 后端不映射端口（%s）⇒ 上面这些 [[端口]] 会被**跳过**，readiness 探测直连 %s:<茧内端口>\n",
			cocoonOrDash(caps.ExposeMechanism), cocoon.DefaultBind)
	}
	// AI 注入面：干跑**只算形状**（✗ 不写占位符件 —— 干跑必须零副作用）。
	if spec.Cocoon.AI.Enabled {
		tokenHost := filepath.Join(cocoonRoot(), "<分配于真跑>", cocoon.AITokenHostFileName)
		if p, _, perr := cocoonAIInject("<分配于真跑>", tokenHost, spec); perr == nil {
			fmt.Fprintf(stdout, "  AI      : enabled · %s（干跑不落件）\n", cocoonAIText(p))
		} else {
			fmt.Fprintf(stdout, "  AI      : enabled · 算不出注入形状：%v（真跑会在起服务之前拒执）\n", perr)
		}
	} else {
		fmt.Fprintf(stdout, "  AI      : %s（契约未开 [茧.ai].enabled ⇒ 不给 AI 能力，是安全侧）\n", cocoonDash)
	}
	fmt.Fprintf(stdout, "  会做的  : 分配茧 id → 写 meta(created→starting) → Provision → 按序起服务 → Expose → 写 meta(running)\n")
	fmt.Fprintf(stdout, "  不会做  : 任何磁盘写入 / 任何进程 / 任何端口\n")
	fmt.Fprintf(stdout, "  状态根  : %s/<分配于真跑>\n", cocoonRoot())
	fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未写 meta、未 Provision、未起服务）")
	return exitOK
}

// ── 后台档验活（open 的判据 · close 的事后验）────────────────────────────

const (
	// cocoonDetachedSettle — 后台档进程要**连续活着**这么久才算「在」。
	//
	// ★ 为什么不是「探一次活着就算」：坏档（entry 是 `/bin/sh -c 'exit 0'`）在 Start 之后
	//   几十毫秒内就没了，而「刚 Start 完」那一瞬所有进程都活着 —— 只探一次必然把「起就崩」
	//   报成成功。这一窗是「不漏判」的代价，真机上是 0.3s 级。
	cocoonDetachedSettle = 300 * time.Millisecond
	// cocoonDetachedBudget — 验活最多等这么久（进程一直在也不许无限等）。
	cocoonDetachedBudget = 5 * time.Second
	// cocoonDetachedStep — 验活轮询的步长。
	cocoonDetachedStep = 50 * time.Millisecond
	// cocoonPidWaitBudget — 等后端把「刚签出的 pid」记进账的上限（只是记账延迟，不涉及就绪）。
	cocoonPidWaitBudget = 2 * time.Second
)

// cocoonLogsCell — 日志格的机器面取值：目录真在就给路径，不在就 `—`（三值纪律：
// 「没有日志」与「日志路径是空的」必须分得开；假后端不起进程 ⇒ 永远没有 logs/）。
func cocoonLogsCell(logDir string) string {
	if st, err := os.Stat(logDir); err == nil && st.IsDir() {
		return logDir
	}
	return cocoonDash
}

// cocoonAwaitPids — 等后端把「刚签出的 pid」记进账（最多 cocoonPidWaitBudget）。
//
// ★ 为什么不能只读一次：`Spawn` 只是把后端 Exec 丢进 goroutine，pid 是**子进程那边**
// 在 Start 之后写进后端记账的 ⇒ 同一条调用栈上立刻读会读到空。这不是「放宽判据」：
// 窗口过后仍是空 ⇒ 调用方按「后端没给 pid」失败（见 cocoonVerifyDetachedAlive）。
func cocoonAwaitPids(pr cocoon.PIDReporter, inst cocoon.Instance) []int {
	deadline := time.Now().Add(cocoonPidWaitBudget)
	for {
		if pids := pr.InstancePIDs(inst); len(pids) > 0 {
			return pids
		}
		if time.Now().After(deadline) {
			return nil
		}
		time.Sleep(cocoonDetachedStep)
	}
}

// cocoonVerifyDetachedAlive — 「后台档的进程真在吗」的判据（open 报成功之前必须过）。
//
// 返回空 = 全在；否则每条是一句人读的失败理由（✗ 第一条就是「进程已退」的那种）。
//
// 三种情形分开处理（三值纪律，✗ 不许把「验不了」混进「验过了」）：
//
//	① 后端没有判活面（DetachedProbe）⇒ **不额外验活**（假后端/测试桩：它们不起进程，
//	   pid 也无从谈起；真后端 nsjail 是实现的，见 backend_nsjail.go 的 DetachedStatus）。
//	② 有判活面却没自报 pid ⇒ **失败**：这是「说得出该验、又拿不出凭据」的形态，
//	   放过它等于把「Start 没报错」当「服务在跑」。
//	③ 有 pid ⇒ 逐条等它连续 settle 时长活着；中途报「已退」⇒ 带退出码/信号失败。
func cocoonVerifyDetachedAlive(be cocoon.Backend, pids []int) []string {
	probe, ok := be.(cocoon.DetachedProbe)
	if !ok {
		return nil
	}
	if len(pids) == 0 {
		return []string{"后端自报支持后台档判活，却没有给出任何 pid ⇒ 无法证明服务在跑（✗ 不许按成功报）"}
	}
	var why []string
	for _, pid := range pids {
		if pid <= 0 {
			why = append(why, fmt.Sprintf("后端给出的 pid 非法（%d）", pid))
			continue
		}
		if msg := cocoonWaitDetachedAlive(probe, pid); msg != "" {
			why = append(why, msg)
		}
	}
	return why
}

// cocoonWaitDetachedAlive — 等一个后台档 pid「真在」；返回空 = 在，否则是失败理由。
func cocoonWaitDetachedAlive(probe cocoon.DetachedProbe, pid int) string {
	deadline := time.Now().Add(cocoonDetachedBudget)
	var aliveSince time.Time
	for {
		alive, exited, res := probe.DetachedStatus(pid)
		if exited {
			return fmt.Sprintf("pid %d 已退出（%s）—— 「起就崩」不是「起好了」", pid, cocoonExitText(res))
		}
		if alive && aliveSince.IsZero() {
			aliveSince = time.Now()
		}
		if !aliveSince.IsZero() && time.Since(aliveSince) >= cocoonDetachedSettle {
			return ""
		}
		if time.Now().After(deadline) {
			// 窗口用完仍未观测到退出 ⇒ 按「在」报（不因为「没攒够一窗」把活着的服务判死）。
			return ""
		}
		time.Sleep(cocoonDetachedStep)
	}
}

// cocoonExitText — 退出观测的人读形（RC / 信号 / 超时三位，缺哪个就不编）。
func cocoonExitText(res cocoon.ExecResult) string {
	switch {
	case res.TimedOut:
		return fmt.Sprintf("超时 RC=%d", res.RC)
	case res.Signal != "":
		return fmt.Sprintf("%s（RC=%d）", res.Signal, res.RC)
	default:
		return fmt.Sprintf("RC=%d", res.RC)
	}
}

// cocoonPortsClosed — `close` 的事后验：契约里那几条茧内端口，此刻必须**连不上**。
//
// 入参是**契约目录**（meta.spec_dir，即 `open <dir>` 那个目录），✗ 不是茧目录 —— 契约不在
// 茧目录里（茧目录是状态根下的 `<id>/`），拿茧目录去读只会得到「读不到 ⇒ 未验」。
//
// 返回 (checked, notes, err)：
//
//	checked=true  ⇒ 每条端口都验过（notes 是逐条读数）
//	checked=false ⇒ 没验（契约读不到 / 契约没写端口），notes 里写明为什么 —— ✗ 不许假装验过
//	err != nil    ⇒ 有端口**还通着**（没停干净）
//
// ★ 为什么要它：pid 收干净 ≠ 端口断了。茧与调用方同网络栈（无 DNAT），端口属主可能是
//
//	茧内孙进程；主进程死了端口还在听的那种残局，只有真 dial 一次才看得见。
//
// ★ 调用点只在「本茧真起过进程」时才调它（见 cmdCocoonClose）：没得验 ≠ 验过了。
func cocoonPortsClosed(specDir string) (checked bool, notes []string, err error) {
	specPath := filepath.Join(specDir, cocoon.SpecFileName)
	raw, rerr := os.ReadFile(specPath)
	if rerr != nil {
		return false, []string{fmt.Sprintf("契约读不到（%s）⇒ 端口面未验", specPath)}, nil
	}
	spec, perr := cocoon.ParseSpecFile(specPath, raw)
	if perr != nil {
		return false, []string{fmt.Sprintf("契约解析不过（%v）⇒ 端口面未验", perr)}, nil
	}
	if len(spec.Ports) == 0 {
		return false, []string{"契约没写 [[端口]] ⇒ 没有可验的端口"}, nil
	}
	var bad []string
	for _, p := range spec.Ports {
		addr := net.JoinHostPort(cocoon.DefaultBind, strconv.Itoa(p.Inner))
		conn, derr := net.DialTimeout("tcp", addr, 300*time.Millisecond)
		if derr != nil {
			notes = append(notes, fmt.Sprintf("%s 不通（已收干净）", addr))
			continue
		}
		_ = conn.Close()
		bad = append(bad, addr)
		notes = append(notes, fmt.Sprintf("%s **仍可连**", addr))
	}
	if len(bad) > 0 {
		return true, notes, fmt.Errorf("close 之后端口仍可连：%s（进程收掉了但端口没断 —— 有残留监听者）",
			strings.Join(bad, ", "))
	}
	return true, notes, nil
}

// ── close ───────────────────────────────────────────────────────────────

// cmdCocoonClose —— `zerg cocoon close <id> [--purge] [--confirm=<id>]`。
//
// 两档：
//
//	缺省   Stop → 写 meta（stopping→gone）→ **留盘**（目录与产物还在）
//	--purge 额外 RemoveCocoon 删树；**必须先 WriteTombstone**（`RemoveCocoon` 内置：墓碑失败 ⇒
//	        ErrNeedTombstone、一个字节都不删），且 `--confirm` 必须与 id **逐字相等**。
//
// ★ 本版后端只有 `fake`：实例句柄按 `{ID: <茧 id>}` 反查（假后端进程内单例 + 跨进程按
// `ErrInstanceNotFound` 当「已经不在了」）。真后端必须另存后端签发的 handle（`Instance` 只能原样
// 传回、✗ 不许自己拼）—— 那是接 nsjail 时的一条待落地面，本件不冒充。
func cmdCocoonClose(inv *invocation, stdout, stderr io.Writer) int {
	if len(inv.args) != 1 {
		inv.setErr("usage", "close_needs_id", "`cocoon close` 要且只要一个茧 id")
		fmt.Fprintf(stderr, "%s: `cocoon close` 要且只要一个茧 id\n", progName)
		fmt.Fprintf(stderr, "用法：%s cocoon close <id> [--purge --confirm=<id>] [--json <字段>]\n", progName)
		return exitUsage
	}
	id := inv.args[0]
	if !cocoonValidID(id) {
		inv.setErr("usage", "bad_cocoon_id", "茧 id 非法")
		fmt.Fprintf(stderr, "%s: 茧 id %q 非法（不许 . / .. / 路径分隔符 / 控制字符）⇒ 退码 2\n", progName, id)
		return exitUsage
	}
	purge := inv.hasFlag("--purge")
	cdir := cocoonMetaDir(id)
	metaPath := cocoon.MetaPath(cdir)
	m, err := cocoon.Load(metaPath)
	if err != nil {
		if errors.Is(err, cocoon.ErrNotFound) {
			if _, terr := cocoon.ReadTombstone(cocoonRoot(), id); terr == nil {
				inv.setErr("failed", "cocoon_already_gone", "只剩墓碑，目录已不在")
				fmt.Fprintf(stderr, "%s: 茧 %s 的目录已不在（只剩墓碑）—— 没有可停/可删的东西 ⇒ 退码 1\n", progName, id)
				return exitFail
			}
			inv.setErr("failed", "cocoon_not_found", "meta 不存在")
			fmt.Fprintf(stderr, "%s: 茧 %s 不存在（%s 读不到）⇒ 退码 1\n", progName, id, metaPath)
			return exitFail
		}
		inv.setErr("failed", "meta_read_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 读 meta 失败：%v ⇒ 退码 1\n", progName, err)
		return exitFail
	}

	// ① 先落 stopping（已经 gone 的不许倒退 —— SetPhase 会拒，见 meta.go），再停。
	if inv.dryRun {
		// 计划件：零副作用（不停、不写 meta、不删）。
		if inv.jsonGiven {
			row := cocoonMetaRow(m, cdir)
			row["phase"], row["purged"] = "planned", strconv.FormatBool(purge)
			row["deleted_files"], row["deleted_dirs"] = "0", "0"
			row["tombstone"], row["verified"] = cocoonDash, "false"
			return listCmd(inv, stdout, stderr, cocoonLsDisplay, []map[string]string{row})
		}
		fmt.Fprintln(stdout, "计划件（--dry-run · 零副作用 —— 未停服务、未写 meta、未删任何东西）")
		fmt.Fprintf(stdout, "  动作    : %s cocoon close\n", progName)
		fmt.Fprintf(stdout, "  茧 id   : %s（phase=%s · pid=%d）\n", m.ID, m.Phase, m.PID)
		fmt.Fprintf(stdout, "  会做的  : 落 stopping → 停/收实例 → 写 gone\n")
		if purge {
			fmt.Fprintf(stdout, "  再删    : 墓碑 %s → 删整棵 %s\n",
				cocoon.FlatMetaPath(cocoonRoot(), id), cdir)
			fmt.Fprintf(stdout, "  删除闸  : 先留墓碑（墓碑失败 ⇒ 零删除）；要 --confirm=%s\n", id)
		} else {
			fmt.Fprintf(stdout, "  留盘    : %s（目录与产物不动）\n", cdir)
		}
		fmt.Fprintf(stdout, "  不会做  : 任何磁盘写入 / 任何删除 / 任何进程动作\n")
		fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未停服务、未写 meta、未删任何东西）")
		return exitOK
	}
	// ★ 确认闸在**任何副作用之前**：`--purge` 缺 `--confirm` / 值不等 ⇒ 零删除、零停服务。
	//   （`--dry-run` 上面已经返回 ⇒ 计划件不需要 confirm；真跑一律要。）
	if purge {
		if !inv.confirmGiven || strings.TrimSpace(inv.confirm) == "" {
			inv.setErr("usage", "purge_needs_confirm", "--purge 必须带 --confirm=<id>")
			fmt.Fprintf(stderr, "%s: `--purge` 会**删整棵茧目录** ⇒ 必须带 `--confirm=<茧 id>`（本次零删除、未停服务）· 退码 2\n", progName)
			fmt.Fprintf(stderr, "先看计划件：%s cocoon close %s --purge --dry-run\n", progName, id)
			return exitUsage
		}
		if inv.confirm != id {
			inv.setErr("usage", "confirm_mismatch", "--confirm 与 id 不等")
			fmt.Fprintf(stderr, "%s: `--confirm=%s` 与茧 id %q **不逐字相等** ⇒ 拒执（本次零删除、未停服务）· 退码 2\n",
				progName, inv.confirm, id)
			return exitUsage
		}
	}
	if m.Phase != cocoon.PhaseGone && m.Phase != cocoon.PhaseDeleting {
		if _, err := cocoon.UpdatePhase(metaPath, cocoon.PhaseStopping); err != nil {
			inv.setErr("failed", "meta_write_failed", err.Error())
			fmt.Fprintf(stderr, "%s: meta 落 stopping 失败（未停服务）：%v ⇒ 退码 1\n", progName, err)
			return exitFail
		}
	}
	// ★ 后端按**盘上那一格**选（meta.backend），不按当前 env 猜：这个茧可能是另一种后端开的
	//   （env 改过一次的机器上，按 env 收尾会收错后端）。老 meta 没这一格 ⇒ 退回当前选档。
	backendName := m.Backend
	if backendName == "" {
		backendName = cocoonBackendName()
	}
	be, berr := cocoonBackendFactory(backendName)
	if berr != nil {
		inv.setErr("failed", "backend_unavailable", berr.Error())
		fmt.Fprintf(stderr, "%s: %v ⇒ 退码 1（meta 停在 stopping）\n", progName, berr)
		return exitFail
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	// ★ 茧目录也交给后端：`close` 在**另一个进程**里，内存记账早没了 ⇒ 靠 meta 的句柄收尾时，
	//   后端要用 BaseDir 算出茧根（`reapRoot` 只删「BaseDir 下一层」的目录，证明不了就不删）。
	//   不设 ⇒ 实例根/upper/merged 收不掉，`close` 只能报成功却留着东西。
	//   目录读不到（已 --purge 过一半）不算失败：这时压根没有茧根可收，后端按「不存在」处理。
	if st, serr := os.Stat(cdir); serr == nil && st.IsDir() {
		if werr := cocoonWireCocoonDir(be, cdir); werr != nil {
			inv.setErr("failed", "backend_unavailable", werr.Error())
			fmt.Fprintf(stderr, "%s: 后端不接受茧目录 %s：%v ⇒ 退码 1（未停服务）\n",
				progName, cdir, werr)
			return exitFail
		}
	}

	// 句柄：后端**签发**的实例 id 原样传回（✗ 不许自己拼 —— 假后端时代那套「拿茧 id 当实例 id」
	// 只在没有 backend_instance 的老 meta 上还留着，是兼容不是口径）。
	inst := cocoon.Instance{ID: id, Backend: backendName, PID: m.PID}
	if m.BackendInstance != "" {
		inst.ID = m.BackendInstance
	}

	reaped := false
	derr := be.Destroy(ctx, inst)
	if errors.Is(derr, cocoon.ErrInstanceNotFound) {
		// 跨进程：内存记账里没有这一号（open 早退场了）⇒ 按盘上句柄真收一遍。
		if r, ok := be.(cocoon.HandleReaper); ok {
			derr = r.ReapPersisted(ctx, inst, m.BackendPIDs)
			reaped = true
		}
	}
	if derr != nil {
		// 假后端（无 HandleReaper）跨进程时没有实例账 ⇒「已经不在了」不算失败；
		// 真后端一律不许用这一条兜底（它必须真的收干净，收不干净就得报）。
		if !errors.Is(derr, cocoon.ErrInstanceNotFound) || reaped {
			inv.setErr("failed", "destroy_failed", derr.Error())
			fmt.Fprintf(stderr, "%s: 停/收茧实例失败：%v ⇒ 退码 1（meta 停在 stopping）\n", progName, derr)
			return exitFail
		}
	}
	cocoonTokenMgr().RevokeToken(id)

	// ★ 事后验两条（缺一条就不算「收干净」）：
	//   ① 进程不在 —— 上面 Destroy/ReapPersisted 已经在验（SIGKILL 后仍在 ⇒ 报错退 1）；
	//   ② 端口不通 —— 只收 pid 证明不了这件事（茧与宿主同网络栈，残留监听者照样能接）。
	//
	// ★ 只在**本茧真起过进程**（meta 里有 pid 表）时验端口：假后端 / 从没起过进程的茧，
	//   端口上有人听也不是它的事 —— 拿它判红是冤枉（三值纪律：没得验就说没得验）。
	portChecked, portNotes, portErr := false, []string{}, error(nil)
	if len(m.BackendPIDs) > 0 {
		portChecked, portNotes, portErr = cocoonPortsClosed(m.SpecDir)
	} else {
		portNotes = []string{"本茧没记 pid（没起过进程）⇒ 端口面不归它，未验"}
	}
	for _, n := range portNotes {
		fmt.Fprintf(stderr, "%s: 端口面：%s\n", progName, n)
	}
	if portErr != nil {
		inv.setErr("failed", "port_still_open", portErr.Error())
		fmt.Fprintf(stderr, "%s: %v ⇒ 退码 1（meta 停在 stopping；收了 pid 但端口还通着）\n", progName, portErr)
		return exitFail
	}

	row := cocoonMetaRow(m, cdir)
	row["purged"] = strconv.FormatBool(false)
	row["deleted_files"] = "0"
	row["deleted_dirs"] = "0"
	row["tombstone"] = cocoonDash
	row["verified"] = "false"
	switch {
	case !portChecked:
		// 没验就说没验（✗ 不把「没得验」写成「验过了」）。
		row["port_verified"] = "skip"
	case len(portNotes) == 0:
		row["port_verified"] = "n/a"
	default:
		row["port_verified"] = "closed"
	}
	if reaped {
		// 收尾方式也进机器面：让人看得出这一次是「按盘上句柄真收」还是「内存记账收」。
		row["reaped"] = "persisted"
	}
	if len(m.BackendPIDs) > 0 {
		fmt.Fprintf(stderr, "%s: 收尾面：后端 %s · 句柄 %s · pid 表 %v（已按盘上句柄收干净）\n",
			progName, backendName, cocoonOrDash(m.BackendInstance), m.BackendPIDs)
	}

	if !purge {
		gone, gerr := cocoon.UpdatePhase(metaPath, cocoon.PhaseGone)
		if gerr != nil {
			inv.setErr("failed", "meta_write_failed", gerr.Error())
			fmt.Fprintf(stderr, "%s: meta 落 gone 失败：%v ⇒ 退码 1\n", progName, gerr)
			return exitFail
		}
		row["phase"] = string(gone.Phase)
		row["dir"] = cdir
		if inv.jsonGiven {
			return listCmd(inv, stdout, stderr, cocoonLsDisplay, []map[string]string{row})
		}
		fmt.Fprintf(stdout, "茧 %s 已停（phase=%s）—— 留盘：%s\n", id, gone.Phase, cdir)
		fmt.Fprintf(stdout, "要连产物一起删：%s cocoon close %s --purge --confirm=%s\n", progName, id, id)
		return exitOK
	}

	// ② purge：deleting → 墓碑 → 删树。已经 gone 的跳过 deleting（monotonic：gone 之后不许倒退）。
	if m.Phase != cocoon.PhaseGone {
		if _, err := cocoon.UpdatePhase(metaPath, cocoon.PhaseDeleting); err != nil {
			inv.setErr("failed", "meta_write_failed", err.Error())
			fmt.Fprintf(stderr, "%s: meta 落 deleting 失败（未删任何东西）：%v ⇒ 退码 1\n", progName, err)
			return exitFail
		}
	}
	cur, rerr := cocoon.Load(metaPath)
	if rerr != nil {
		inv.setErr("failed", "meta_read_failed", rerr.Error())
		fmt.Fprintf(stderr, "%s: 回读 meta 失败（未删任何东西）：%v ⇒ 退码 1\n", progName, rerr)
		return exitFail
	}
	// 茧根身份：与删除同一套打开方式取（`StatRoot`），拿不到就**不绑身份**（Report 会写 verified=false）。
	want, serr := cocoon.StatRoot(cocoonRoot(), id)
	if serr != nil {
		if _, terr := os.Stat(cdir); errors.Is(terr, os.ErrNotExist) {
			// 目录已经不在：留墓碑即可（这是「删净」的最后一步，不是失败）。
			cur.Phase = cocoon.PhaseGone
			if err := cocoon.WriteTombstone(cocoonRoot(), cur); err != nil {
				inv.setErr("failed", "tombstone_failed", err.Error())
				fmt.Fprintf(stderr, "%s: 留墓碑失败：%v ⇒ 退码 1\n", progName, err)
				return exitFail
			}
			row["phase"] = string(cocoon.PhaseGone)
			row["purged"] = strconv.FormatBool(true)
			row["tombstone"] = cocoon.FlatMetaPath(cocoonRoot(), id)
			if inv.jsonGiven {
				return listCmd(inv, stdout, stderr, cocoonLsDisplay, []map[string]string{row})
			}
			fmt.Fprintf(stdout, "茧 %s 的目录已不在 —— 只补了墓碑：%s\n", id, row["tombstone"])
			return exitOK
		}
		inv.setErr("failed", "root_stat_failed", serr.Error())
		fmt.Fprintf(stderr, "%s: 取茧根身份失败（未删任何东西）：%v ⇒ 退码 1\n", progName, serr)
		return exitFail
	}
	rep, rmErr := cocoon.RemoveCocoon(cocoonRoot(), cur, want)
	row["deleted_files"] = strconv.Itoa(rep.Files)
	row["deleted_dirs"] = strconv.Itoa(rep.Dirs)
	row["verified"] = strconv.FormatBool(rep.Verified)
	row["tombstone"] = cocoon.FlatMetaPath(cocoonRoot(), id)
	if rmErr != nil {
		if errors.Is(rmErr, cocoon.ErrNeedTombstone) {
			inv.setErr("failed", "tombstone_failed", rmErr.Error())
			fmt.Fprintf(stderr, "%s: 留墓碑失败 ⇒ **零删除**（目录一字节未动）：%v · 退码 1\n", progName, rmErr)
			return exitFail
		}
		inv.setErr("failed", "purge_failed", rmErr.Error())
		fmt.Fprintf(stderr, "%s: 删茧目录失败（已删 %d 文件 / %d 目录，停在这里、后面没动）：%v ⇒ 退码 1\n",
			progName, rep.Files, rep.Dirs, rmErr)
		return exitFail
	}
	// ③ 墓碑改写 gone（删净之后的结论）—— 墓碑件在 root/<id>.meta.json，与目录无关。
	cur.Phase = cocoon.PhaseGone
	if err := cocoon.WriteTombstone(cocoonRoot(), cur); err != nil {
		inv.setErr("failed", "tombstone_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 删净了，但墓碑改写 gone 失败：%v ⇒ 退码 1\n", progName, err)
		return exitFail
	}
	row["phase"] = string(cocoon.PhaseGone)
	row["purged"] = strconv.FormatBool(true)
	row["dir"] = cdir
	if inv.jsonGiven {
		return listCmd(inv, stdout, stderr, cocoonLsDisplay, []map[string]string{row})
	}
	fmt.Fprintf(stdout, "茧 %s 已停并删净（phase=%s · 删 %d 文件 / %d 目录 · 身份锚定=%t）\n",
		id, cocoon.PhaseGone, rep.Files, rep.Dirs, rep.Verified)
	fmt.Fprintf(stdout, "墓碑     : %s（删后唯一的凭据；对账/reconcile 靠它）\n", row["tombstone"])
	if rep.Degraded != "" {
		fmt.Fprintf(stderr, "%s: ⚠ 删除降级：%s\n", progName, rep.Degraded)
	}
	return exitOK
}

// ── ls ──────────────────────────────────────────────────────────────────

// cmdCocoonLs —— `zerg cocoon ls [--json <字段>]`。
//
// ★ 只列 **meta 能读出来** 的茧（真源 = meta.json）。目录在而 meta 没有 ⇒ 孤儿：
// 本命令**不列、不删**（只往 stderr 报一个计数 —— 不给它一个「茧」的身份）。
func cmdCocoonLs(inv *invocation, stdout, stderr io.Writer) int {
	root := cocoonRoot()
	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// 还没有任何茧：空清单（rc=0），✗ 不把「没建过」当错。
			return listCmd(inv, stdout, stderr, cocoonLsDisplay, nil)
		}
		inv.setErr("failed", "root_unreadable", err.Error())
		fmt.Fprintf(stderr, "%s: 读不了茧状态根 %s：%v ⇒ 退码 1\n", progName, root, err)
		return exitFail
	}
	rows := []map[string]string{}
	orphans, broken := 0, 0
	for _, e := range entries {
		if !e.IsDir() {
			continue // 平铺面是墓碑（<id>.meta.json），不是活着的茧
		}
		id := e.Name()
		if !cocoonValidID(id) {
			continue
		}
		cdir := filepath.Join(root, id)
		m, lerr := cocoonReadMeta(cocoon.MetaPath(cdir))
		if lerr != nil {
			if errors.Is(lerr, cocoon.ErrNotFound) {
				orphans++ // 有目录无 meta ⇒ 孤儿，归 reconcile 处置
			} else {
				broken++ // meta 坏了也是「判不了」，✗ 不当成茧列出来
			}
			continue
		}
		if m.ID != id {
			broken++ // meta 里的 id 与目录名分家 ⇒ 不给它一个身份
			continue
		}
		rows = append(rows, cocoonMetaRow(m, cdir))
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i]["id"] < rows[j]["id"] })
	// 孤儿/坏账只报**计数**（stderr）：它们不是茧 —— 清单里不许出现，也不许静默吞掉。
	if orphans > 0 || broken > 0 {
		fmt.Fprintf(stderr, "%s: 跳过 %d 个无 meta 的目录 + %d 个 meta 读不出的目录（孤儿/坏账，归 reconcile 处置；✗ 不进清单）\n",
			progName, orphans, broken)
	}
	if inv.jsonGiven {
		return listCmd(inv, stdout, stderr, cocoonLsDisplay, rows)
	}
	if len(rows) == 0 {
		fmt.Fprintf(stdout, "（没有茧 · 状态根 %s）\n", root)
	} else {
		listCmd(inv, stdout, stderr, cocoonLsDisplay, rows)
	}
	return exitOK
}

// ── inspect ─────────────────────────────────────────────────────────────

// cmdCocoonInspect —— `zerg cocoon inspect <id> [--json <字段>]`。
//
// **只读**：meta + 后端能力面 + 注入面（AI env 形状）。不写盘（连锁件都不建 —— 见 `cocoonReadMeta`）。
// 令牌面只出脱敏形：真值不落盘，`ZERG_COCOON_TOKEN` 给了也只回 `RedactToken` 的产物。
func cmdCocoonInspect(inv *invocation, stdout, stderr io.Writer) int {
	if len(inv.args) != 1 {
		inv.setErr("usage", "inspect_needs_id", "`cocoon inspect` 要且只要一个茧 id")
		fmt.Fprintf(stderr, "%s: `cocoon inspect` 要且只要一个茧 id\n", progName)
		fmt.Fprintf(stderr, "用法：%s cocoon inspect <id> [--json <字段>]\n", progName)
		return exitUsage
	}
	id := inv.args[0]
	if !cocoonValidID(id) {
		inv.setErr("usage", "bad_cocoon_id", "茧 id 非法")
		fmt.Fprintf(stderr, "%s: 茧 id %q 非法（不许 . / .. / 路径分隔符）⇒ 退码 2\n", progName, id)
		return exitUsage
	}
	cdir := cocoonMetaDir(id)
	m, err := cocoonReadMeta(cocoon.MetaPath(cdir))
	if err != nil {
		if errors.Is(err, cocoon.ErrNotFound) {
			inv.setErr("failed", "cocoon_not_found", "meta 不存在")
			fmt.Fprintf(stderr, "%s: 茧 %s 不存在（%s 读不到）⇒ 退码 1\n", progName, id, cocoon.MetaPath(cdir))
			return exitFail
		}
		inv.setErr("failed", "meta_read_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 读 meta 失败：%v ⇒ 退码 1\n", progName, err)
		return exitFail
	}
	// 后端名：**meta 里那一格**优先（这才是「这个茧实际用的后端」）；没登记（老 meta）才退回当前选档。
	backendName := m.Backend
	if backendName == "" {
		backendName = cocoonBackendName()
	}
	row := cocoonMetaRow(m, cdir)
	row["backend"] = backendName
	row["backend_instance"] = cocoonOrDash(m.BackendInstance)

	// 能力面：问后端自己（`CapabilityReporter`）。★ 观察面**不拒**：拿不到就如实印「不可用」，
	// 其余字段照常 —— 因能力面读不出来就整份不看，等于把「怎么坏的」也一起藏了。
	// ✗ 也不许填空值冒充一张「全 false 的表」（那会被读成「后端在，只是弱」）。
	var caps cocoon.Capabilities
	capsErr := error(nil)
	if be, berr := cocoonBackendFactory(backendName); berr != nil {
		capsErr = berr
	} else {
		caps, capsErr = cocoonBackendCaps(be)
	}
	if capsErr != nil {
		for _, k := range []string{"platform", "isolation", "peer_mechanism", "kill_mechanism",
			"expose", "expose_mechanism"} {
			row[k] = cocoonDash
		}
		row["peer_identity"] = cocoonDash
		for _, k := range []string{"process_tree_kill", "symlink_nofollow", "cross_mount_guard"} {
			row[k] = "false"
		}
		row["caps_error"] = capsErr.Error()
	} else {
		row["platform"] = cocoonOrDash(caps.Platform)
		row["isolation"] = cocoonOrDash(caps.Isolation)
		row["peer_identity"] = string(caps.PeerIdentity)
		row["peer_mechanism"] = cocoonOrDash(caps.PeerMechanism)
		row["process_tree_kill"] = strconv.FormatBool(caps.ProcessTreeKill)
		row["kill_mechanism"] = cocoonOrDash(caps.KillMechanism)
		row["symlink_nofollow"] = strconv.FormatBool(caps.SymlinkNoFollow)
		row["cross_mount_guard"] = strconv.FormatBool(caps.CrossMountGuard)
		row["expose"] = strconv.FormatBool(caps.Expose)
		row["expose_mechanism"] = cocoonOrDash(caps.ExposeMechanism)
		row["caps_error"] = cocoonDash
	}

	// 注入面：真源是**契约 `[茧.ai]`**（`open` 就是按它算 env 的），✗ 不是 meta ——
	// meta 里一个令牌面都不落。契约读不到 / 没开这一段 ⇒ 三格如实印 `—`（✗ 编一个形状）。
	//
	// ★ 这里**不读令牌件本身**：它的内容恒为占位符（读它没有新信息），而"读一次"会让
	// inspect 从"只读 meta+契约"变成"还要摸茧目录里的件"，白白多一条会失败的路径。
	aiSpec, _ := cocoonSpecOfMeta(m)
	if aiSpec != nil && aiSpec.Cocoon.AI.Enabled {
		if plan, _, perr := cocoonAIInject(m.ID, cocoon.AITokenHostPath(cdir), aiSpec); perr == nil {
			row["ai_base"] = plan.Base
			row["ai_model"] = plan.Model
			row["ai_env"] = strings.Join(plan.Environ(), ";")
		} else {
			row["ai_base"], row["ai_model"], row["ai_env"] = cocoonDash, cocoonDash, cocoonDash
		}
	} else {
		row["ai_base"], row["ai_model"], row["ai_env"] = cocoonDash, cocoonDash, cocoonDash
	}

	tokenText, tokenSource := cocoonDash, "absent"
	if raw := strings.TrimSpace(os.Getenv(cocoonTokenEnv)); raw != "" {
		tokenText, tokenSource = cocoon.RedactToken(raw), "env:"+cocoonTokenEnv
	}
	row["token"] = tokenText
	row["token_source"] = tokenSource

	// 判据 9（v2 §E）：契约哈希锁 —— **只读比对、只报告不拒**（inspect 是观察面：因为契约漂了
	// 就不许看别的，等于把「怎么坏的」也一起藏了）。三格：锁值 / 现值 / 判定（闭集四值）。
	lock := cocoonSpecLockOf(m)
	row["spec_lock"] = lock.State
	row["spec_sha"] = cocoonOrDash(lock.Locked)
	row["spec_now"] = cocoonOrDash(lock.Current)

	if inv.jsonGiven {
		return listCmd(inv, stdout, stderr, cocoonLsDisplay, []map[string]string{row})
	}
	fmt.Fprintf(stdout, "茧 id      : %s\n", m.ID)
	fmt.Fprintf(stdout, "相位       : %s\n", m.Phase)
	fmt.Fprintf(stdout, "pid        : %d\n", m.PID)
	fmt.Fprintf(stdout, "端口       : %s\n", cocoonPortsText(m.Ports))
	fmt.Fprintf(stdout, "绑定的卵   : %s\n", cocoonOrDash(m.Egg))
	fmt.Fprintf(stdout, "归属       : %s · 建于 %s\n", cocoonOrDash(m.Owner), cocoonStamp(m.CreatedAt))
	if m.StartTime != nil {
		fmt.Fprintf(stdout, "起于       : %s\n", cocoonStamp(*m.StartTime))
	}
	fmt.Fprintf(stdout, "状态根     : %s\n", cdir)
	if capsErr != nil {
		fmt.Fprintf(stdout, "后端       : %s · 能力面不可用：%v\n", backendName, capsErr)
	} else {
		fmt.Fprintf(stdout, "后端       : %s（句柄 %s）· 平台 %s · 隔离 %s\n",
			backendName, cocoonOrDash(m.BackendInstance), cocoonOrDash(caps.Platform), cocoonOrDash(caps.Isolation))
		fmt.Fprintf(stdout, "能力面     : 对端身份=%s（%s）· 整树回收=%t（%s）· 不跟链接=%t · 不跨挂载=%t\n",
			caps.PeerIdentity, cocoonOrDash(caps.PeerMechanism), caps.ProcessTreeKill,
			cocoonOrDash(caps.KillMechanism), caps.SymlinkNoFollow, caps.CrossMountGuard)
		fmt.Fprintf(stdout, "端口面     : 映射=%t（%s）\n", caps.Expose, cocoonOrDash(caps.ExposeMechanism))
		// 可写层（rootfs 的 overlay）：与「端口面」同口径 —— 位 + 理由缺一不可。
		// false 不等于「不能换根」：只能 chroot 的后端照样能跑只读镜像（见 Capabilities.Overlay）。
		fmt.Fprintf(stdout, "可写层     : overlay=%t（%s）\n", caps.Overlay, cocoonOrDash(caps.OverlayMechanism))
	}
	if row["ai_base"] != cocoonDash && row["ai_base"] != "" {
		fmt.Fprintf(stdout, "AI 注入面  : base=%s model=%s env=%s\n",
			row["ai_base"], row["ai_model"], strings.ReplaceAll(row["ai_env"], ";", " · "))
	} else {
		fmt.Fprintf(stdout, "AI 注入面  : %s（契约未开 [茧.ai].enabled，或契约读不到 ⇒ 没有 AI 能力，安全侧）\n", cocoonDash)
	}
	fmt.Fprintf(stdout, "令牌       : %s（%s · 只出脱敏形）\n", tokenText, tokenSource)
	// 契约锁：人面只印前 8 位（全文在 meta.json 的 spec_sha —— 要看全文去读那件，不往终端抄指纹）。
	switch lock.State {
	case "ok":
		fmt.Fprintf(stdout, "契约锁     : ok（锁定 %s… = 现读 %s…）\n",
			cocoon.SpecShort(lock.Locked), cocoon.SpecShort(lock.Current))
	case "drift":
		fmt.Fprintf(stdout, "契约锁     : ⚠ 漂移（锁定 %s… ≠ 现读 %s…）—— %s\n",
			cocoon.SpecShort(lock.Locked), cocoon.SpecShort(lock.Current), lock.Detail)
	case "unreadable":
		fmt.Fprintf(stdout, "契约锁     : ⚠ 判不了（契约读不到）—— %s\n", lock.Detail)
	default:
		fmt.Fprintf(stdout, "契约锁     : 未锁定（meta 里没有 spec_sha/spec_dir ⇒ 老茧或脚本写入 · ✗ 不当成一致）\n")
	}
	return exitOK
}

// ── bind ────────────────────────────────────────────────────────────────

// cmdCocoonBind —— `zerg cocoon bind <id> <卵名>`：把茧绑到一颗虫卵（写 meta 的 `egg` 字段）。
//
// 写面：先读（拿同一份 meta）→ 改 `Egg` → 原子 Save。不绑已删/删了一半的茧（phase 是终态 ⇒ 拒）。
func cmdCocoonBind(inv *invocation, stdout, stderr io.Writer) int {
	if len(inv.args) != 2 {
		inv.setErr("usage", "bind_needs_two_args", "`cocoon bind` 要两个位置参数")
		fmt.Fprintf(stderr, "%s: `cocoon bind` 要两个位置参数：<茧 id> <卵名>\n", progName)
		fmt.Fprintf(stderr, "用法：%s cocoon bind <id> <卵名> [--json <字段>]\n", progName)
		return exitUsage
	}
	id := inv.args[0]
	egg := strings.TrimSpace(inv.args[1])
	if !cocoonValidID(id) {
		inv.setErr("usage", "bad_cocoon_id", "茧 id 非法")
		fmt.Fprintf(stderr, "%s: 茧 id %q 非法（不许 . / .. / 路径分隔符）⇒ 退码 2\n", progName, id)
		return exitUsage
	}
	if egg == "" || egg == "." || egg == ".." || strings.ContainsAny(egg, "\r\n\t") {
		inv.setErr("usage", "bad_egg_name", "卵名非法")
		fmt.Fprintf(stderr, "%s: 卵名 %q 非法（不许空 / . / .. / 换行制表符）⇒ 退码 2\n", progName, egg)
		return exitUsage
	}
	cdir := cocoonMetaDir(id)
	metaPath := cocoon.MetaPath(cdir)
	m, err := cocoon.Load(metaPath)
	if err != nil {
		if errors.Is(err, cocoon.ErrNotFound) {
			inv.setErr("failed", "cocoon_not_found", "meta 不存在")
			fmt.Fprintf(stderr, "%s: 茧 %s 不存在（%s 读不到）⇒ 退码 1\n", progName, id, metaPath)
			return exitFail
		}
		inv.setErr("failed", "meta_read_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 读 meta 失败：%v ⇒ 退码 1\n", progName, err)
		return exitFail
	}
	if m.Phase == cocoon.PhaseGone || m.Phase == cocoon.PhaseDeleting {
		inv.setErr("failed", "cocoon_closed", "茧已在终态，不能再绑卵")
		fmt.Fprintf(stderr, "%s: 茧 %s 的 phase=%s 已是终态 ⇒ 不能再绑卵（退码 1）\n", progName, id, m.Phase)
		return exitFail
	}
	prev := m.Egg
	m.Egg = egg
	if err := cocoon.Save(metaPath, m); err != nil {
		inv.setErr("failed", "meta_write_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 写 meta 失败：%v ⇒ 退码 1\n", progName, err)
		return exitFail
	}
	row := cocoonMetaRow(m, cdir)
	row["egg"] = egg
	if inv.jsonGiven {
		return listCmd(inv, stdout, stderr, cocoonLsDisplay, []map[string]string{row})
	}
	if prev == "" {
		fmt.Fprintf(stdout, "茧 %s 已绑到卵 %s（此前未绑）\n", id, egg)
	} else {
		fmt.Fprintf(stdout, "茧 %s 已改绑：%s → %s\n", id, prev, egg)
	}
	fmt.Fprintf(stdout, "状态根     : %s\n", cdir)
	return exitOK
}
