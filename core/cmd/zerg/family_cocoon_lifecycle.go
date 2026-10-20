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
// 未接的面（照实登记 · 不当已做）：真镜像 rootfs（nsjail 后端只做 namespace/cgroup 隔离，
// `--chroot` 缺省是宿主根；镜像落盘/overlay 未接）· 伪终端（ExecOpts.TTY=true 一律拒）·
// VZ/HCS 两个后端 · 令牌真值落点（本版只签发并在输出里给脱敏形）。
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	// cocoonAIUpstreamEnv / cocoonDefaultAIUpstream —— 注入面的宿主 broker 回环 base。
	cocoonAIUpstreamEnv     = "ZERG_AI_UPSTREAM"
	cocoonDefaultAIUpstream = "http://127.0.0.1:8082"

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

	cocoonOpenFields = []string{"id", "phase", "pid", "ports", "egg", "backend", "token", "dir"}

	cocoonCloseFields = []string{"id", "phase", "purged", "deleted_files", "deleted_dirs",
		"tombstone", "verified", "dir", "reaped"}

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

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// 服务的 stdout/stderr 落 <茧目录>/logs/<服务>.log：起服务的是**本进程**，而本进程会退出
	// —— 管道读端随之消失，长驻服务下一次写就会撞 EPIPE（见 cocoon.ExecOpts.LogPath）。
	opts := cocoon.OrchestrateOptions{LogDir: filepath.Join(cdir, "logs")}
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

	// ★ 后端**签发**的句柄落盘（实例 id + 活 pid 表）：`close` 是另一个进程，内存记账那时已经没了。
	//   这是「跨进程收干净」的唯一依据 —— 不落它，close 只能报成功却什么都没收。
	var pids []int
	if pr, ok := be.(cocoon.PIDReporter); ok {
		pids = pr.InstancePIDs(inst)
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
		pidNote = fmt.Sprintf("（后端签发的 exec 主进程：%v · 已落 meta.backend_pids）", m2.BackendPIDs)
	}
	fmt.Fprintf(stdout, "pid        : %d%s\n", m2.PID, pidNote)
	if caps.Expose {
		fmt.Fprintf(stdout, "端口       : %s\n", cocoonPortsText(ports))
	} else if len(spec.Ports) > 0 {
		fmt.Fprintf(stdout, "端口       : %s（后端不映射：直连 %s:<茧内端口>；理由见 stderr）\n",
			cocoonDash, cocoon.DefaultBind)
	} else {
		fmt.Fprintf(stdout, "端口       : %s\n", cocoonPortsText(ports))
	}
	fmt.Fprintf(stdout, "服务       : %s\n", strings.Join(spec.Order, " → "))
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
	fmt.Fprintf(stdout, "  会做的  : 分配茧 id → 写 meta(created→starting) → Provision → 按序起服务 → Expose → 写 meta(running)\n")
	fmt.Fprintf(stdout, "  不会做  : 任何磁盘写入 / 任何进程 / 任何端口\n")
	fmt.Fprintf(stdout, "  状态根  : %s/<分配于真跑>\n", cocoonRoot())
	fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未写 meta、未 Provision、未起服务）")
	return exitOK
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

	row := cocoonMetaRow(m, cdir)
	row["purged"] = strconv.FormatBool(false)
	row["deleted_files"] = "0"
	row["deleted_dirs"] = "0"
	row["tombstone"] = cocoonDash
	row["verified"] = "false"
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

	// 注入面：绑了卵才有 AI 形状（没绑 ⇒ 零值 plan = 没有 AI 能力，是安全侧）。
	upstream := strings.TrimSpace(os.Getenv(cocoonAIUpstreamEnv))
	if upstream == "" {
		upstream = cocoonDefaultAIUpstream
	}
	plan := cocoon.BuildAIEnv(m.ID, m.Egg, upstream)
	if plan.Valid() {
		row["ai_base"] = plan.Base
		row["ai_model"] = plan.Model
		row["ai_env"] = strings.Join(plan.Environ(), ";")
	} else {
		row["ai_base"] = cocoonDash
		row["ai_model"] = cocoonDash
		row["ai_env"] = cocoonDash
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
	}
	if plan.Valid() {
		fmt.Fprintf(stdout, "AI 注入面  : base=%s model=%s env=%s\n",
			plan.Base, plan.Model, strings.Join(plan.Environ(), " · "))
	} else {
		fmt.Fprintf(stdout, "AI 注入面  : %s（未绑定卵 ⇒ 没有 AI 能力，安全侧）\n", cocoonDash)
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
