// family_cocoon_broker.go —— 宿主 broker 的命令面（T4c）：
//
//	zerg cocoon broker start  [--dry-run] [--json <字段>]
//	zerg cocoon broker stop   [--dry-run] [--json <字段>]
//	zerg cocoon broker status [--json <字段>]
//
// ★ 为什么要有这一条（T4b 实测的两个「开箱即坏」）：
//
//	① broker 的缺省地址原先是 `127.0.0.1:8082` —— 那正是 `zerg-core` 网关的**正式端口**
//	   （`core/cmd/zerg-core/main.go` 的 `gw.Start(8082)`）⇒ 本机 bind 直接 `EADDRINUSE`，
//	   broker 缺省起不来。T4c 把缺省让到 `127.0.0.1:8092`（`cocoon.DefaultBrokerHTTPAddr`），
//	   并且**只留一个旋钮** `ZERG_BROKER_ADDR`：它同时决定 ① 监听地址 ② 茧内缺省 `ZERG_AI_BASE`
//	   的 host:port（旧名 `ZERG_BROKER_HTTP_ADDR` 当同义回退 · 两处仍同一个真源）。
//	② 本仓此前**没有起 broker 的命令面** ⇒ 茧内拿到的 `ZERG_AI_BASE` 指向一扇没人开的门
//	   （真值的交接在 `cocoon.Broker` 里做得很完整，但生产上没人能请动它）。本件补的就是这一段。
//
// 口径（逐条都是判据，不是文风）：
//
//	① **成功不许假**：`start` 只有等到 `/health` **真 200 且 body 的 `ok=true`、`upstream` 与本进程
//	   解析出来的上游逐字相等**才返回 0。超时 / 子进程提前退出 / 端口被别人占 ⇒ 一律 1 并点名
//	   （✗ 不许「进程起了就当成功」—— 那正是要修的那类假绿）。
//	② **脱钩与 open 服务同口径**：`Setsid: true`（自成会话）＋ stdin `/dev/null` ＋
//	   stdout/stderr 落 `broker.log`（O_APPEND 0644），照 `ui_start.go` 的 `uiStartLaunch`。
//	③ **一个宿主一份 broker**：状态落在 `<状态目录>/broker/`（`broker.pid` ＋ `broker.json` ＋
//	   `broker.log`）。`start` 幂等（已在跑 ⇒ 报 pid 后回 0，不重起）；`stop` 按 pid 收：
//	   SIGTERM → 宽限 → SIGKILL → **事后验端口不通**（再报残留 / 占用者）。
//	④ **`--dry-run` 零副作用**：不绑端口、不写 pid 件/状态件、不起进程、不开日志；计划件必须印出
//	   **实际将用的 addr**（含来源：`default` / `env:ZERG_BROKER_ADDR` / `env:ZERG_BROKER_HTTP_ADDR`）。
//	⑤ **只开 HTTP 回环面**（`cocoon.ServeHTTPFace`）：`/health` ＋ `POST /v1/chat/completions`。
//	   unix 面（每茧独立 uid / peer 判据）**本版没接** —— 它的授权面是另一张契约，✗ 不许在这里
//	   假装接了（对端强度照 `broker_http.go` 的「未接的面」记 `none`）。
//	⑥ **真令牌只在 broker 进程内**：上游令牌从宿主 env `ZERG_AI_UPSTREAM_TOKEN` 取（给了才装
//	   `SecretResolver`，并同时进 `ScrubSecrets`）；✗ 不落盘、不进状态件、不进输出。
//
// 退码闭集 0/1/2（与 `family_cocoon_lifecycle.go` 同一口径；本件不新开号）：
//
//	0 成功（`status`：broker 在跑且 `/health` 通）· 1 失败 / 状态不是「在跑」· 2 用法错或配置非法
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/cocoon"
	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

const (
	// cocoonBrokerServeEnv —— **内部**标记：子进程以「前台服务」档跑（✗ 不是命令旗标）。
	//
	// ★ 为什么用 env 而不是 `--serve` 旗标：旗标要进 `main.go` 的旗标名表 ＋ usage 串
	// （门73 面③/面⑤ 双向判），而这一档对人是**不可见的能力**（它只由 `start` 在 fork 时注入）。
	// 用 env 就不给命令面开第二个入口。
	cocoonBrokerServeEnv = "ZERG_BROKER_SERVE"
	// cocoonBrokerTokenEnv —— 可选的上游令牌（宿主 env）。给了才装 `SecretResolver`。
	cocoonBrokerTokenEnv = "ZERG_AI_UPSTREAM_TOKEN"

	// cocoonBrokerStartTimeout —— `start` 等 `/health` 真 200 的上限。
	cocoonBrokerStartTimeout = 15 * time.Second
	// cocoonBrokerStopGrace —— `stop` 给 SIGTERM 的宽限（过了才 SIGKILL）。
	cocoonBrokerStopGrace = 5 * time.Second
	// cocoonBrokerTick —— 轮询间隔（`start` 等就绪 / `stop` 等退出 / 两个端口探测）。
	cocoonBrokerTick = 100 * time.Millisecond
	// cocoonBrokerDialTimeout —— 单个 TCP/HTTP 探测的上限（本机回环，1 秒足够）。
	cocoonBrokerDialTimeout = 800 * time.Millisecond
	// cocoonBrokerLogTail —— 失败时回给人的日志尾巴行数（不整份倒出来）。
	cocoonBrokerLogTail = 12

	// 三个动作词（位置参数闭集）。
	cocoonBrokerStart  = "start"
	cocoonBrokerStop   = "stop"
	cocoonBrokerStatus = "status"
)

// cocoonBrokerFields —— `--json` 可取的全部字段（`status` 至少含 addr/pid/upstream/model）。
var cocoonBrokerFields = []string{"state", "addr", "addr_source", "pid", "upstream", "model",
	"health", "pid_file", "log"}

// ── 落点 ────────────────────────────────────────────────────────────────

func cocoonBrokerDir() string       { return filepath.Join(statepath.Dir(), "broker") }
func cocoonBrokerPIDFile() string   { return filepath.Join(cocoonBrokerDir(), "broker.pid") }
func cocoonBrokerStateFile() string { return filepath.Join(cocoonBrokerDir(), "broker.json") }
func cocoonBrokerLogFile() string   { return filepath.Join(cocoonBrokerDir(), "broker.log") }

// cocoonBrokerState —— 落盘的运行态（由**子进程**在 `ListenHTTP` 成功之后写，`stop`/`status` 只读）。
type cocoonBrokerState struct {
	PID        int    `json:"pid"`
	Addr       string `json:"addr"`
	AddrSource string `json:"addr_source"`
	Upstream   string `json:"upstream"`
	Model      string `json:"model"`
	StartedAt  string `json:"started_at"`
	Exe        string `json:"exe"`
}

// ── 运行态读写 ──────────────────────────────────────────────────────────

// cocoonBrokerReadState —— 读状态件；读不到 / 解析不动 ⇒ (nil,false)（观察面不抛错）。
func cocoonBrokerReadState() (*cocoonBrokerState, bool) {
	raw, err := os.ReadFile(cocoonBrokerStateFile())
	if err != nil {
		return nil, false
	}
	var st cocoonBrokerState
	if jerr := json.Unmarshal(raw, &st); jerr != nil || st.PID <= 0 {
		return nil, false
	}
	return &st, true
}

// cocoonBrokerWriteState —— 原子写（唯一临时名 ＋ rename ⇒ 半个件永不被读到）。
func cocoonBrokerWriteState(st *cocoonBrokerState) error {
	if st == nil {
		return errors.New("cocoon: broker 状态为空")
	}
	if err := os.MkdirAll(cocoonBrokerDir(), 0o700); err != nil {
		return err
	}
	body, err := json.MarshalIndent(st, "", " ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	f, err := os.CreateTemp(cocoonBrokerDir(), "broker-json-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if _, werr := f.Write(body); werr != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return werr
	}
	if cerr := f.Close(); cerr != nil {
		_ = os.Remove(tmp)
		return cerr
	}
	if rerr := os.Rename(tmp, cocoonBrokerStateFile()); rerr != nil {
		_ = os.Remove(tmp)
		return rerr
	}
	return os.WriteFile(cocoonBrokerPIDFile(), []byte(strconv.Itoa(st.PID)+"\n"), 0o600)
}

// cocoonBrokerClearState —— 清 pid 件与状态件（幂等；件不在也不算错）。
func cocoonBrokerClearState() {
	_ = os.Remove(cocoonBrokerPIDFile())
	_ = os.Remove(cocoonBrokerStateFile())
}

// cocoonBrokerAlive —— pid 是否还活着（`kill 0`：EPERM 也算活着）。
func cocoonBrokerAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// ── 端口面 ──────────────────────────────────────────────────────────────

// cocoonBrokerPortFree —— 真试着 bind 一次再放开：`EADDRINUSE` 是唯一能证明「有人占着」的证据。
func cocoonBrokerPortFree(addr string) bool {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

// cocoonBrokerDialOK —— 能否连上（`stop` 事后验端口用；连得上 = 还没收干净）。
func cocoonBrokerDialOK(addr string) bool {
	c, err := net.DialTimeout("tcp", addr, cocoonBrokerDialTimeout)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}

// cocoonBrokerOwner —— 端口占用者一格（名字 ＋ pid）。
type cocoonBrokerOwner struct {
	Name string
	PID  int
}

// cocoonBrokerOwners —— 问 `lsof` 谁在 LISTEN 这个端口（拿不到 ⇒ 空切片，✗ 不猜）。
//
// ★ 只用它**报人**（`start` 的负控要「点名哪个进程占的」）：判定「起没起来」一律走 `/health`
// 与 pid 件 —— 拿 lsof 当判据的话，lsof 不在盘上（精简镜像）时会把「在跑」读成「没人占」。
func cocoonBrokerOwners(addr string) []cocoonBrokerOwner {
	_, port, err := net.SplitHostPort(addr)
	if err != nil || strings.TrimSpace(port) == "" {
		return nil
	}
	out, lerr := exec.Command("lsof", "-nP", "-iTCP:"+port, "-sTCP:LISTEN", "-F", "pc").Output()
	if lerr != nil {
		return nil
	}
	var res []cocoonBrokerOwner
	cur := cocoonBrokerOwner{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "p"):
			if n, aerr := strconv.Atoi(line[1:]); aerr == nil {
				cur.PID = n
			}
		case strings.HasPrefix(line, "c"):
			cur.Name = strings.TrimSpace(line[1:])
			if cur.PID > 0 {
				res = append(res, cur)
			}
			cur = cocoonBrokerOwner{}
		}
	}
	return res
}

// cocoonBrokerOwnerText —— 人面那一格（无占用者 ⇒ 空串，✗ 不写「未知」冒充已查）。
func cocoonBrokerOwnerText(addr string) string {
	owns := cocoonBrokerOwners(addr)
	if len(owns) == 0 {
		return ""
	}
	parts := make([]string, 0, len(owns))
	for _, o := range owns {
		name := o.Name
		if name == "" {
			name = "（lsof 没给名字）"
		}
		parts = append(parts, fmt.Sprintf("%s(pid=%d)", name, o.PID))
	}
	return strings.Join(parts, " · ")
}

// ── /health ─────────────────────────────────────────────────────────────

// cocoonBrokerHealthRow —— `/health` 的一行读数（`ok` 为真才算「这一面真在服务」）。
type cocoonBrokerHealthRow struct {
	OK       bool
	Backend  string
	Model    string
	Upstream string
}

// cocoonBrokerHealth —— 现拨 `/health`；`ok=false` 且 err 非 nil 才表示「拨不到」。
//
// ★ 不走环境代理（`Proxy: nil`）：本机回环不该经过 `HTTP_PROXY` —— 一个健康探针顺手把请求
// 交给代理，会在代理慢的时候把「broker 没起」读成「broker 卡住」。
func cocoonBrokerHealth(addr string) (cocoonBrokerHealthRow, error) {
	cl := &http.Client{
		Timeout:   cocoonBrokerDialTimeout,
		Transport: &http.Transport{Proxy: nil},
	}
	resp, err := cl.Get("http://" + addr + "/health")
	if err != nil {
		return cocoonBrokerHealthRow{}, err
	}
	defer resp.Body.Close()
	var body struct {
		OK       bool   `json:"ok"`
		Backend  string `json:"backend"`
		Model    string `json:"model"`
		Upstream string `json:"upstream"`
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, 64<<10))
	if derr := dec.Decode(&body); derr != nil {
		return cocoonBrokerHealthRow{}, fmt.Errorf("/health 回了 %d 但不是 JSON：%v", resp.StatusCode, derr)
	}
	if resp.StatusCode != http.StatusOK {
		return cocoonBrokerHealthRow{}, fmt.Errorf("/health 回了 %d（要 200）", resp.StatusCode)
	}
	return cocoonBrokerHealthRow{OK: body.OK, Backend: body.Backend, Model: body.Model, Upstream: body.Upstream}, nil
}

// ── 配置解析（与 broker 面同一个真源）────────────────────────────────────

// cocoonBrokerConfig —— 这一轮实际要用的 (addr, 上游, 模型, 令牌面)；**只从 cocoon 的真源取**。
type cocoonBrokerConfig struct {
	Addr       string
	AddrSource string
	Upstream   string
	Model      string
	Token      string // 真值：只在 serve 档用，✗ 不进任何输出/落盘
}

// cocoonBrokerResolve — 一处算全（addr 走 `cocoon.ResolveBrokerAddr`、上游走 `cocoon.ResolveAIUpstream`）。
func cocoonBrokerResolve() (cocoonBrokerConfig, error) {
	var cfg cocoonBrokerConfig
	addr, source, aerr := cocoon.ResolveBrokerAddr()
	if aerr != nil {
		return cfg, aerr
	}
	up, uerr := cocoon.ResolveAIUpstream("")
	if uerr != nil {
		return cfg, uerr
	}
	cfg.Addr = addr
	cfg.AddrSource = source
	cfg.Upstream = up
	cfg.Model = cocoon.DefaultCocoonAIModel
	cfg.Token = strings.TrimSpace(os.Getenv(cocoonBrokerTokenEnv))
	return cfg, nil
}

// ── 入口 ────────────────────────────────────────────────────────────────

// cmdCocoonBroker —— `zerg cocoon broker <start|stop|status>`。
//
// 三条动作分派到这里；动作词是**位置参数**（`inv.args[0]`），不是命令树的一段 ——
// 于是命令树只多一条 path（`cocoon broker`），三条动作共用同一张字段表与同一份用法串。
func cmdCocoonBroker(inv *invocation, stdout, stderr io.Writer) int {
	// serve 档（子进程）：`start` ＋ 内部标记 env ⇒ 前台真服务。
	if strings.TrimSpace(os.Getenv(cocoonBrokerServeEnv)) == "1" && len(inv.args) > 0 && inv.args[0] == cocoonBrokerStart && !inv.dryRun {
		return cocoonBrokerServe(inv, stdout, stderr)
	}
	if len(inv.args) != 1 {
		inv.setErr("usage", "broker_needs_action", "`cocoon broker` 要且只要一个动作词（start|stop|status）")
		fmt.Fprintf(stderr, "%s: `cocoon broker` 要且只要一个动作词：start | stop | status\n", progName)
		fmt.Fprintf(stderr, "用法：%s cocoon broker <start|stop|status> [--dry-run] [--json <字段>]\n", progName)
		return exitUsage
	}
	switch inv.args[0] {
	case cocoonBrokerStart:
		return cocoonBrokerStartCmd(inv, stdout, stderr)
	case cocoonBrokerStop:
		return cocoonBrokerStopCmd(inv, stdout, stderr)
	case cocoonBrokerStatus:
		return cocoonBrokerStatusCmd(inv, stdout, stderr)
	}
	inv.setErr("usage", "broker_action_unknown", "动作词不在闭集 start|stop|status")
	fmt.Fprintf(stderr, "%s: 动作词 %q 不在闭集（start | stop | status）⇒ 退码 2\n", progName, inv.args[0])
	fmt.Fprintf(stderr, "用法：%s cocoon broker <start|stop|status> [--dry-run] [--json <字段>]\n", progName)
	return exitUsage
}

// ── start ───────────────────────────────────────────────────────────────

func cocoonBrokerStartCmd(inv *invocation, stdout, stderr io.Writer) int {
	var human strings.Builder
	cfg, cerr := cocoonBrokerResolve()
	if cerr != nil {
		inv.setErr("usage", "broker_config_invalid", cerr.Error())
		fmt.Fprintf(stderr, "%s: broker 配置算不出来（地址旋钮 %s／%s 或上游 %s 非法）：%v ⇒ 退码 2\n",
			progName, cocoon.EnvBrokerAddr, cocoon.EnvBrokerHTTPAddr, cocoon.EnvAIUpstream, cerr)
		return exitUsage
	}
	base := "http://" + cfg.Addr

	// 已在跑（pid 活着 ＋ /health 真 200）⇒ 幂等：报 pid 后回 0，不重起。
	if st, ok := cocoonBrokerReadState(); ok && cocoonBrokerAlive(st.PID) {
		if h, herr := cocoonBrokerHealth(cfg.Addr); herr == nil && h.OK {
			fmt.Fprintf(&human, "broker 已在跑（pid=%d · addr=%s · 上游 %s · 模型 %s）⇒ 不重复起\n",
				st.PID, cfg.Addr, h.Upstream, h.Model)
			return cocoonBrokerFinish(inv, stdout, stderr, &human, cocoonBrokerRow("up", cfg, st.PID, "ok", &h))
		}
	}

	if inv.dryRun {
		cocoonBrokerPlan(&human, "start", cfg)
		return cocoonBrokerFinish(inv, stdout, stderr, &human, cocoonBrokerRow("plan", cfg, 0, "未探测", nil))
	}

	// 端口占用预检：占着就**点名**是谁，✗ 不许「先起再说」。
	if !cocoonBrokerPortFree(cfg.Addr) {
		// 若占着的正是 pid 件里那个进程（半起：进程在、/health 不通）⇒ 那是本命令自己留下的残局。
		stale := ""
		if st, ok := cocoonBrokerReadState(); ok && cocoonBrokerAlive(st.PID) {
			stale = fmt.Sprintf("（pid 件里是 pid=%d，进程还活着但 /health 不通 ⇒ 半起，先 `%s cocoon broker stop`）",
				st.PID, progName)
		}
		owner := cocoonBrokerOwnerText(cfg.Addr)
		if owner == "" {
			owner = "（占用者未定位：本机没有 lsof 或它读不到）"
		}
		if h, herr := cocoonBrokerHealth(cfg.Addr); herr == nil && h.OK {
			owner += fmt.Sprintf("；但 /health 是 200（ok=%t · 上游 %s）—— 已经有一个 broker 在这一格，只是没有 pid 件",
				h.OK, h.Upstream)
		}
		inv.setErr("failed", "broker_port_busy", "端口已被占用："+cfg.Addr)
		fmt.Fprintf(stderr, "%s: broker 起不来 —— %s 已被占用：%s%s\n", progName, cfg.Addr, owner, stale)
		fmt.Fprintf(stderr, "  换一格：%s=<host:port> %s cocoon broker start（一个旋钮同时改监听与茧内 base）\n",
			cocoon.EnvBrokerAddr, progName)
		return cocoonBrokerFinish(inv, stdout, stderr, &human, cocoonBrokerRow("port_busy", cfg, 0, "不可达", nil), exitFail)
	}

	exe, eerr := os.Executable()
	if eerr != nil {
		inv.setErr("failed", "broker_self_exe_failed", eerr.Error())
		fmt.Fprintf(stderr, "%s: 取不到自身可执行件（子进程起不来）：%v ⇒ 退码 1\n", progName, eerr)
		return exitFail
	}
	if merr := os.MkdirAll(cocoonBrokerDir(), 0o700); merr != nil {
		inv.setErr("failed", "broker_state_dir_failed", merr.Error())
		fmt.Fprintf(stderr, "%s: 建状态目录失败（%s）：%v ⇒ 退码 1\n", progName, cocoonBrokerDir(), merr)
		return exitFail
	}
	logf, lerr := os.OpenFile(cocoonBrokerLogFile(), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if lerr != nil {
		inv.setErr("failed", "broker_log_failed", lerr.Error())
		fmt.Fprintf(stderr, "%s: 开不了日志件（%s）：%v ⇒ 退码 1\n", progName, cocoonBrokerLogFile(), lerr)
		return exitFail
	}
	devnull, derr := os.OpenFile(os.DevNull, os.O_RDONLY, 0)
	if derr != nil {
		_ = logf.Close()
		inv.setErr("failed", "broker_devnull_failed", derr.Error())
		fmt.Fprintf(stderr, "%s: 开不了 %s：%v ⇒ 退码 1\n", progName, os.DevNull, derr)
		return exitFail
	}

	// 脱钩（与 open 服务 / ui start 同口径）：Setsid 自成会话 ⇒ CLI 退出不带走 broker。
	child := exec.Command(exe, "cocoon", "broker", cocoonBrokerStart)
	child.Stdin = devnull
	child.Stdout = logf
	child.Stderr = logf
	child.Env = append(cocoonBrokerEnvSet(os.Environ(), cocoonBrokerServeEnv, ""), cocoonBrokerServeEnv+"=1")
	child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	startErr := child.Start()
	_ = logf.Close()
	_ = devnull.Close()
	if startErr != nil {
		inv.setErr("failed", "broker_spawn_failed", startErr.Error())
		fmt.Fprintf(stderr, "%s: 起不动 broker 子进程：%v ⇒ 退码 1（日志 %s）\n",
			progName, startErr, cocoonBrokerLogFile())
		return exitFail
	}
	pid := child.Process.Pid
	// 回收（✗ 留僵尸）：Setsid 只保证「不随会话被清理」，不保证父进程不收拾它。
	go func() { _ = child.Wait() }()

	// ★ 唯一的成功判据：/health 真 200 ＋ ok=true ＋ upstream 与本进程解析的一致。
	deadline := time.Now().Add(cocoonBrokerStartTimeout)
	var lastErr error
	for time.Now().Before(deadline) {
		h, herr := cocoonBrokerHealth(cfg.Addr)
		if herr == nil && h.OK {
			if h.Upstream != cfg.Upstream {
				inv.setErr("failed", "broker_health_mismatch", "健康面报的上游与解析值不同")
				fmt.Fprintf(stderr, "%s: %s 上的 /health 通了但不是本命令起的那个 broker —— 上游报 %q，本进程解析 %q ⇒ 退码 1\n",
					progName, cfg.Addr, h.Upstream, cfg.Upstream)
				cocoonBrokerKillQuiet(pid)
				return cocoonBrokerFinish(inv, stdout, stderr, &human, cocoonBrokerRow("health_mismatch", cfg, pid, "上游不符", &h), exitFail)
			}
			// ★ 判据不止 /health：状态件必须由**子进程**写好且 pid 对得上 ——
			//   `stop`/`status` 全靠它；「health 通了但没人写下 pid」等于给下一条命令留了个瞎点。
			if st, ok := cocoonBrokerReadState(); !ok || st.PID != pid {
				inv.setErr("failed", "broker_state_missing", "子进程没写下状态件")
				fmt.Fprintf(stderr, "%s: /health 通了，但状态件（%s）没写或 pid 对不上（要 %d）⇒ 不收这一趟，退码 1\n",
					progName, cocoonBrokerStateFile(), pid)
				cocoonBrokerKillQuiet(pid)
				cocoonBrokerClearState()
				return cocoonBrokerFinish(inv, stdout, stderr, &human, cocoonBrokerRow("state_missing", cfg, pid, "ok", &h), exitFail)
			}
			fmt.Fprintf(&human, "broker 起来了：pid=%d · addr=%s（%s）· 茧内 base=%s · 上游 %s · 模型 %s\n",
				pid, cfg.Addr, cfg.AddrSource, base, h.Upstream, h.Model)
			fmt.Fprintf(stderr, "（/health 已真 200；收工：%s cocoon broker stop）\n", progName)
			return cocoonBrokerFinish(inv, stdout, stderr, &human, cocoonBrokerRow("up", cfg, pid, "ok", &h))
		}
		if herr != nil {
			lastErr = herr
		}
		if !cocoonBrokerAlive(pid) {
			// 子进程自己退了（多半是 bind 失败）⇒ 别等满超时，直接把它的日志尾巴交给人。
			inv.setErr("failed", "broker_child_exited", "子进程在就绪前退出")
			fmt.Fprintf(stderr, "%s: broker 子进程（pid=%d）在 /health 就绪之前就退出了 ⇒ 退码 1\n", progName, pid)
			cocoonBrokerDumpLogTail(stderr)
			return cocoonBrokerFinish(inv, stdout, stderr, &human, cocoonBrokerRow("exited", cfg, pid, "不可达", nil), exitFail)
		}
		time.Sleep(cocoonBrokerTick)
	}

	// 超时：收掉自己起的那个进程，✗ 不许留一个「起了但没通」的半成品占着端口。
	cocoonBrokerKillQuiet(pid)
	inv.setErr("failed", "broker_health_timeout", "等 /health 200 超时")
	if lastErr != nil {
		fmt.Fprintf(stderr, "%s: 等了 %s 也没等到 %s/health 的 200（最后一次：%v）⇒ 退码 1（已收掉 pid=%d）\n",
			progName, cocoonBrokerStartTimeout, base, lastErr, pid)
	} else {
		fmt.Fprintf(stderr, "%s: 等了 %s 也没等到 %s/health 的 200 ⇒ 退码 1（已收掉 pid=%d）\n",
			progName, cocoonBrokerStartTimeout, base, pid)
	}
	cocoonBrokerDumpLogTail(stderr)
	return cocoonBrokerFinish(inv, stdout, stderr, &human, cocoonBrokerRow("timeout", cfg, pid, "不可达", nil), exitFail)
}

// ── serve（子进程前台档）──────────────────────────────────────────────

// cocoonBrokerServe —— 前台把 broker 跑起来（只由 `start` 经 `ZERG_BROKER_SERVE=1` 进入）。
//
// 顺序（每一步都 fail-closed）：解析配置 → `ListenHTTP`（bind 回环）→ **写状态件/pid 件**
// → `ServeHTTPFace` 直到 SIGTERM/SIGINT。★ 状态件在 bind **之后**写 ⇒ 「有 pid 件」意味着
// 「这一面真绑上了」，不是「进程起来了」。
func cocoonBrokerServe(inv *invocation, stdout, stderr io.Writer) int {
	cfg, cerr := cocoonBrokerResolve()
	if cerr != nil {
		fmt.Fprintf(stderr, "%s: broker 配置算不出来：%v\n", progName, cerr)
		return exitUsage
	}
	b := cocoon.NewBroker(nil)
	b.Ops = nil // unix 面本版没接；HTTP 面不看 Ops（两条路径写死，见 aiHandler）
	if cfg.Token != "" {
		tok := cfg.Token
		b.Secrets = cocoon.FuncSecretResolver(func(string) (string, error) { return tok, nil })
		b.ScrubSecrets = []string{tok}
	}
	opts := cocoon.BrokerHTTPOptions{Addr: cfg.Addr, Upstream: cfg.Upstream, Model: cfg.Model}
	actual, lerr := b.ListenHTTP(opts)
	if lerr != nil {
		fmt.Fprintf(stderr, "%s: broker 绑不上 %s：%v\n", progName, cfg.Addr, lerr)
		return exitFail
	}
	defer func() { _ = b.Close() }()

	pid := os.Getpid()
	exe, _ := os.Executable()
	st := &cocoonBrokerState{
		PID: pid, Addr: actual, AddrSource: cfg.AddrSource, Upstream: cfg.Upstream,
		Model: cfg.Model, StartedAt: time.Now().UTC().Format(time.RFC3339), Exe: exe,
	}
	if werr := cocoonBrokerWriteState(st); werr != nil {
		fmt.Fprintf(stderr, "%s: 写 broker 状态件失败（%s）：%v\n", progName, cocoonBrokerStateFile(), werr)
		return exitFail
	}
	// 收工时只清**自己那一份**（✗ 不删别人的 pid 件：并发 start 时那是别人的凭据）。
	defer func() {
		if cur, ok := cocoonBrokerReadState(); ok && cur.PID == pid {
			cocoonBrokerClearState()
		}
	}()

	fmt.Fprintf(stderr, "%s: broker 前台服务 pid=%d addr=%s upstream=%s model=%s\n",
		progName, pid, actual, cfg.Upstream, cfg.Model)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sig)
	go func() {
		select {
		case <-sig:
			cancel()
		case <-ctx.Done():
		}
	}()

	if serr := b.ServeHTTPFace(ctx); serr != nil {
		fmt.Fprintf(stderr, "%s: broker HTTP 面停了：%v\n", progName, serr)
		return exitFail
	}
	fmt.Fprintf(stderr, "%s: broker 收工（pid=%d）\n", progName, pid)
	return exitOK
}

// ── stop ────────────────────────────────────────────────────────────────

func cocoonBrokerStopCmd(inv *invocation, stdout, stderr io.Writer) int {
	var human strings.Builder
	cfg, cerr := cocoonBrokerResolve()
	if cerr != nil {
		inv.setErr("usage", "broker_config_invalid", cerr.Error())
		fmt.Fprintf(stderr, "%s: broker 配置算不出来（地址旋钮非法）：%v ⇒ 退码 2\n", progName, cerr)
		return exitUsage
	}
	st, ok := cocoonBrokerReadState()
	if !ok {
		// 没有状态件 ⇒ 本命令没起过它。**端口若被占，绝不代杀**（那是别人的进程）。
		if cocoonBrokerDialOK(cfg.Addr) {
			owner := cocoonBrokerOwnerText(cfg.Addr)
			if owner == "" {
				owner = "（占用者未定位）"
			}
			inv.setErr("failed", "broker_unmanaged_port", "端口被非本命令的进程占用")
			fmt.Fprintf(stderr, "%s: 没有 broker 状态件（%s），但 %s 是通的、占用者：%s ⇒ 不碰别人的进程，退码 1\n",
				progName, cocoonBrokerStateFile(), cfg.Addr, owner)
			return cocoonBrokerFinish(inv, stdout, stderr, &human, cocoonBrokerRow("unmanaged", cfg, 0, "可达", nil), exitFail)
		}
		fmt.Fprintf(&human, "broker 未在跑（没有状态件 · %s 不通）⇒ 无事可做\n", cfg.Addr)
		return cocoonBrokerFinish(inv, stdout, stderr, &human, cocoonBrokerRow("down", cfg, 0, "不可达", nil))
	}

	if inv.dryRun {
		alive := "已退出"
		if cocoonBrokerAlive(st.PID) {
			alive = "活着"
		}
		fmt.Fprintf(&human, "计划件（--dry-run · 零副作用）\n")
		fmt.Fprintf(&human, "  动作    : %s cocoon broker stop\n", progName)
		fmt.Fprintf(&human, "  目标 pid: %d（%s）\n", st.PID, alive)
		fmt.Fprintf(&human, "  地址    : %s（来源 %s）\n", st.Addr, st.AddrSource)
		fmt.Fprintf(&human, "  信号    : SIGTERM → 宽限 %s → SIGKILL（只在还活着时补刀）\n", cocoonBrokerStopGrace)
		fmt.Fprintf(&human, "  收工后  : 验 %s 不通 ＋ 清 %s\n", st.Addr, cocoonBrokerStateFile())
		fmt.Fprintf(&human, "  不会做  : 不发任何信号 / 不删任何件\n")
		fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未发信号、未删件）")
		return cocoonBrokerFinish(inv, stdout, stderr, &human, cocoonBrokerRow("plan", cfg, st.PID, "未探测", nil))
	}

	pid := st.PID
	if !cocoonBrokerAlive(pid) {
		// 进程早已不在（崩了 / 被别的手段收了）⇒ 只清残件，随后仍要验端口。
		fmt.Fprintf(&human, "broker 进程（pid=%d）早已不在 ⇒ 只清残件\n", pid)
		cocoonBrokerClearState()
	} else {
		if kerr := syscall.Kill(pid, syscall.SIGTERM); kerr != nil && !errors.Is(kerr, syscall.ESRCH) {
			inv.setErr("failed", "broker_signal_failed", kerr.Error())
			fmt.Fprintf(stderr, "%s: 给 pid=%d 发 SIGTERM 失败：%v ⇒ 退码 1\n", progName, pid, kerr)
			return cocoonBrokerFinish(inv, stdout, stderr, &human, cocoonBrokerRow("signal_failed", cfg, pid, "未知", nil), exitFail)
		}
		if !cocoonBrokerWaitExit(pid, cocoonBrokerStopGrace) {
			fmt.Fprintf(stderr, "%s: pid=%d 过 %s 还没退 ⇒ SIGKILL\n", progName, pid, cocoonBrokerStopGrace)
			if kerr := syscall.Kill(pid, syscall.SIGKILL); kerr != nil && !errors.Is(kerr, syscall.ESRCH) {
				inv.setErr("failed", "broker_signal_failed", kerr.Error())
				fmt.Fprintf(stderr, "%s: SIGKILL 也发不出去（pid=%d）：%v ⇒ 退码 1\n", progName, pid, kerr)
				return cocoonBrokerFinish(inv, stdout, stderr, &human, cocoonBrokerRow("signal_failed", cfg, pid, "未知", nil), exitFail)
			}
			if !cocoonBrokerWaitExit(pid, cocoonBrokerStopGrace) {
				inv.setErr("failed", "broker_survived_kill", "SIGKILL 之后进程仍在")
				fmt.Fprintf(stderr, "%s: ⚠ pid=%d 收到 SIGKILL 之后仍在（可能卡在内核态 / 或被别的监督面拉起）⇒ 退码 1\n",
					progName, pid)
				return cocoonBrokerFinish(inv, stdout, stderr, &human, cocoonBrokerRow("survived", cfg, pid, "不可达", nil), exitFail)
			}
		}
		cocoonBrokerClearState()
	}

	// ★ 事后验：端口必须**不通**，且不留进程。两条都过才算 stop 成功。
	if cocoonBrokerAlive(pid) {
		inv.setErr("failed", "broker_survived_kill", "进程仍在")
		fmt.Fprintf(stderr, "%s: ⚠ pid=%d 仍在 ⇒ 退码 1\n", progName, pid)
		return cocoonBrokerFinish(inv, stdout, stderr, &human, cocoonBrokerRow("survived", cfg, pid, "不可达", nil), exitFail)
	}
	if cocoonBrokerDialOK(st.Addr) {
		owner := cocoonBrokerOwnerText(st.Addr)
		if owner == "" {
			owner = "（占用者未定位：没有 lsof 或它读不到）"
		}
		inv.setErr("failed", "broker_port_still_open", "pid 收了但端口仍通")
		fmt.Fprintf(stderr, "%s: ⚠ pid=%d 已收，但 %s 仍能连上 —— 占用者：%s ⇒ 退码 1\n",
			progName, pid, st.Addr, owner)
		return cocoonBrokerFinish(inv, stdout, stderr, &human, cocoonBrokerRow("port_open", cfg, pid, "可达", nil), exitFail)
	}
	fmt.Fprintf(&human, "broker 已停：pid=%d 已退出 · %s 不通 · 状态件已清\n", pid, st.Addr)
	return cocoonBrokerFinish(inv, stdout, stderr, &human, cocoonBrokerRow("down", cfg, 0, "不可达", nil))
}

// ── status ──────────────────────────────────────────────────────────────

func cocoonBrokerStatusCmd(inv *invocation, stdout, stderr io.Writer) int {
	var human strings.Builder
	if inv.dryRun {
		inv.setErr("usage", "broker_dry_run_na", "status 是只读面，--dry-run 不适用")
		fmt.Fprintf(stderr, "%s: `cocoon broker status` 是只读面，`--dry-run` 不适用（它本来零副作用）⇒ 退码 2\n", progName)
		return exitUsage
	}
	cfg, cerr := cocoonBrokerResolve()
	if cerr != nil {
		inv.setErr("usage", "broker_config_invalid", cerr.Error())
		fmt.Fprintf(stderr, "%s: broker 配置算不出来（地址旋钮非法）：%v ⇒ 退码 2\n", progName, cerr)
		return exitUsage
	}
	st, hasState := cocoonBrokerReadState()
	pid := 0
	if hasState {
		pid = st.PID
	}
	alive := cocoonBrokerAlive(pid)
	h, herr := cocoonBrokerHealth(cfg.Addr)
	healthOK := herr == nil && h.OK

	state := "down"
	switch {
	case alive && healthOK:
		state = "up"
	case alive && !healthOK:
		state = "degraded"
	case !alive && (healthOK || cocoonBrokerDialOK(cfg.Addr)):
		// 端口有人应答但不是 pid 件里那个进程（或根本没有 pid 件）⇒ 不是本命令起的。
		state = "unmanaged"
	}

	row := cocoonBrokerRow(state, cfg, pid, cocoonBrokerHealthWord(herr, h), &h)

	// 人面：**一行**（脚本/人扫一眼就够；细节那几格留给 --json）。
	switch state {
	case "up":
		fmt.Fprintf(&human, "broker: up · addr=%s · pid=%d · upstream=%s · model=%s\n",
			cfg.Addr, pid, h.Upstream, h.Model)
	case "degraded":
		fmt.Fprintf(stderr, "%s: pid=%d 在，但 %s/health 不通（%v）⇒ 半起，先 `%s cocoon broker stop`\n",
			progName, pid, "http://"+cfg.Addr, herr, progName)
		fmt.Fprintf(&human, "broker: degraded · addr=%s · pid=%d · upstream=%s · model=%s\n",
			cfg.Addr, pid, cfg.Upstream, cfg.Model)
	case "unmanaged":
		owner := cocoonBrokerOwnerText(cfg.Addr)
		if owner == "" {
			owner = "（占用者未定位）"
		}
		fmt.Fprintf(&human, "broker: unmanaged · addr=%s · 占用者=%s · upstream=%s · model=%s\n",
			cfg.Addr, owner, cfg.Upstream, cfg.Model)
	default:
		fmt.Fprintf(&human, "broker: down · addr=%s（来源 %s）· upstream=%s · model=%s\n",
			cfg.Addr, cfg.AddrSource, cfg.Upstream, cfg.Model)
	}

	if state == "up" {
		return cocoonBrokerFinish(inv, stdout, stderr, &human, row)
	}
	return cocoonBrokerFinish(inv, stdout, stderr, &human, row, exitFail)
}

// ── 小工具 ──────────────────────────────────────────────────────────────

// cocoonBrokerRow —— 三档共用的一行（`--json` 的真源；`pid<=0` ⇒ 该键整键略去，✗ 不落 0）。
func cocoonBrokerRow(state string, cfg cocoonBrokerConfig, pid int, health string, h *cocoonBrokerHealthRow) map[string]string {
	row := map[string]string{
		"state":       state,
		"addr":        cfg.Addr,
		"addr_source": cfg.AddrSource,
		"health":      health,
		"pid_file":    cocoonBrokerPIDFile(),
		"log":         cocoonBrokerLogFile(),
	}
	if pid > 0 {
		row["pid"] = strconv.Itoa(pid)
	}
	// 在跑 ⇒ 报**健康面**的真值（它才是这一面实际用的上游/模型）；没在跑 ⇒ 报解析值
	// （「它会拨哪个上游」—— 那一格必须可答，否则 status 在 down 档等于什么都不说）。
	if h != nil && h.OK {
		row["upstream"] = h.Upstream
		row["model"] = h.Model
	} else {
		row["upstream"] = cfg.Upstream
		row["model"] = cfg.Model
	}
	return row
}

// cocoonBrokerHealthWord —— `health` 那一格的字面（✗ 不把「拨不到」写成「ok」）。
func cocoonBrokerHealthWord(herr error, h cocoonBrokerHealthRow) string {
	if herr != nil {
		return "不可达"
	}
	if h.OK {
		return "ok"
	}
	return "非 ok"
}

// cocoonBrokerFinish —— 唯一出口：`--json` 档 stdout **只许**有包封（★ 人面绝不混进去）。
//
// ★ 为什么人面先攒到 `human` 里而不是直接印：`--json` 的消费方要的是**可解析的一件**；
//
//	人面那一行混进 stdout 会让整份 JSON 变成 `invalid character 'b' looking for beginning of value`。
//	⇒ 人面进缓冲，出口一次判：给 `--json` ⇒ 只出包封；不给 ⇒ 只出人面（判词一律走 stderr）。
func cocoonBrokerFinish(inv *invocation, stdout, stderr io.Writer, human *strings.Builder, row map[string]string, rc ...int) int {
	code := exitOK
	if len(rc) > 0 {
		code = rc[0]
	}
	if inv.jsonGiven {
		if jrc := selectJSON(stdout, stderr, inv, inv.path, inv.fields, row); jrc != exitOK {
			return jrc
		}
		return code
	}
	if human != nil && human.Len() > 0 {
		_, _ = io.WriteString(stdout, human.String())
	}
	return code
}

// cocoonBrokerPlan —— `start --dry-run` 的计划件（★ 必须印**实际将用的 addr** ＋ 它的来源）。
func cocoonBrokerPlan(w io.Writer, action string, cfg cocoonBrokerConfig) {
	fmt.Fprintf(w, "计划件（--dry-run · 零副作用）\n")
	fmt.Fprintf(w, "  动作    : %s cocoon broker %s\n", progName, action)
	fmt.Fprintf(w, "  地址    : %s（%s）\n", cfg.Addr, cfg.AddrSource)
	fmt.Fprintf(w, "  茧内base: http://%s（茧内缺省 ZERG_AI_BASE = 同一个真源）\n", cfg.Addr)
	fmt.Fprintf(w, "  上游    : %s\n", cfg.Upstream)
	fmt.Fprintf(w, "  模型    : %s\n", cfg.Model)
	fmt.Fprintf(w, "  令牌面  : %s\n", cocoonBrokerTokenWord(cfg))
	fmt.Fprintf(w, "  pid 件  : %s\n", cocoonBrokerPIDFile())
	fmt.Fprintf(w, "  状态件  : %s\n", cocoonBrokerStateFile())
	fmt.Fprintf(w, "  日志    : %s\n", cocoonBrokerLogFile())
	fmt.Fprintf(w, "  会做的  : 起一个脱钩子进程（Setsid）→ 绑 %s → 等 /health 真 200 才回 0\n", cfg.Addr)
	fmt.Fprintf(w, "  不会做  : 不绑端口 / 不写 pid 件与状态件 / 不起进程 / 不写日志\n")
	fmt.Fprintf(w, "  旋钮    : %s（新名）· %s（旧名 · 同义回退）\n", cocoon.EnvBrokerAddr, cocoon.EnvBrokerHTTPAddr)
}

// cocoonBrokerTokenWord —— 令牌**面**（✗ 绝不出真值，只出「装没装」）。
func cocoonBrokerTokenWord(cfg cocoonBrokerConfig) string {
	if cfg.Token == "" {
		return "none（未设 " + cocoonBrokerTokenEnv + " ⇒ 出站不加认证头）"
	}
	return "broker:env:" + cocoonBrokerTokenEnv + "（真值只在 broker 进程内 · 不进任何输出）"
}

// cocoonBrokerEnvSet —— 设/覆盖环境表里的键（`v==""` ⇒ 删掉该键）；✗ 不改入参。
func cocoonBrokerEnvSet(env []string, k, v string) []string {
	pfx := k + "="
	out := make([]string, 0, len(env)+1)
	for _, e := range env {
		if strings.HasPrefix(e, pfx) {
			continue
		}
		out = append(out, e)
	}
	if v != "" {
		out = append(out, pfx+v)
	}
	return out
}

// cocoonBrokerWaitExit —— 等进程退出（`kill 0` 打不通即「退了」）。
func cocoonBrokerWaitExit(pid int, within time.Duration) bool {
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if !cocoonBrokerAlive(pid) {
			return true
		}
		time.Sleep(cocoonBrokerTick)
	}
	return !cocoonBrokerAlive(pid)
}

// cocoonBrokerKillQuiet —— 收掉自己刚起的那个 pid（先 TERM、宽限后 KILL）；✗ 不报错给用户。
func cocoonBrokerKillQuiet(pid int) {
	if pid <= 0 || !cocoonBrokerAlive(pid) {
		return
	}
	_ = syscall.Kill(pid, syscall.SIGTERM)
	if !cocoonBrokerWaitExit(pid, cocoonBrokerStopGrace) {
		_ = syscall.Kill(pid, syscall.SIGKILL)
		_ = cocoonBrokerWaitExit(pid, cocoonBrokerStopGrace)
	}
}

// cocoonBrokerDumpLogTail —— 失败时把那一段日志尾巴交给人（✗ 整份倒出：broker 日志会长）。
func cocoonBrokerDumpLogTail(w io.Writer) {
	raw, err := os.ReadFile(cocoonBrokerLogFile())
	if err != nil || len(raw) == 0 {
		return
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) > cocoonBrokerLogTail {
		lines = lines[len(lines)-cocoonBrokerLogTail:]
	}
	fmt.Fprintf(w, "  日志尾巴（%s · 末 %d 行）：\n", cocoonBrokerLogFile(), len(lines))
	for _, ln := range lines {
		fmt.Fprintf(w, "    %s\n", ln)
	}
}
