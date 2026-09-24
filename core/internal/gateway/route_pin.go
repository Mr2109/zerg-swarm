// route_pin.go —— 选机前那道**「查显式指定」的门**（单独定制 > 路由默认规则 · 2026-09-24）。
//
// 病灶（现读可证的现场）：子代理没法把模型钉到指定机器 —— 网关择优**默认挑本机**，
// 请求落在 Mr2109 的 `llama-server`（物证 `ps`：`llama-server -m ~/models/example-35b-v2-Q4_K_M.gguf`），
// 而人要的是 x3 那枚；当时只有「手工 `POST /api/control/unload {"machine":"Mr2109"}` 绕路」这一条路。
//
// 本件落地的是**优先级写死的四级**（评审铁律，逐条可测）：
//
//	① 请求头 `X-Zerg-Machine: <机器>` —— **一次性**（只作用于本次请求）· **零落盘**（一个字节都不写）
//	② 覆盖表 `<状态目录>/route_pins.json` —— 带 **TTL**（`expires_at`，过期即不再命中）· 由 `zerg route pin|unpin` 写
//	③ 命中 ①/② ⇒ **命中即用**（不去打分、不做轮询）
//	④ 都没命中 ⇒ **逐字走 pickRoute 原有逻辑**（本文件一个字都不参与）——这是「默认规则不变」的可验证形式
//
// 命中即用时**不豁免**的两条（照既有红线）：
//
//	· **目标机必须是该模型的候选**：不是 ⇒ fail-closed（`model X has no candidate on host Y`），
//	  **绝不悄悄回默认择优** —— 那正好是「钉住等于没钉」的静默降级。
//	· **能力硬门照走**（`gateRoute`）：与 DS4 让位那条强制本机路径**同一口径**
//	  （「绝不因让位而静默降级」的原文见 `capability_gate.go` 文件头）。
//
// 命中即用时**不适用**的两条（这就是「命中即用」的含义，如实写在日志里，不装作没这回事）：
//
//	· 熔断（T7）与活性过滤**不参与判决** —— 人点名的那台就是那台（否则钉住会被择优悄悄改道）；
//	  两枚信号（是否熔断 / 有没有心跳快照）照打日志，**看得见**，不静默。
//	· 轮询/账本/cache-aware 打分**不参与**（没有候选可比 ⇒ 没有择优可言）。
//
// ★ 只读：本文件**没有任何写盘调用**（0 个 `os.WriteFile` / `os.Create` / `os.OpenFile` /
//
//	`os.Rename` / `os.Remove` / `os.MkdirAll` —— 由 `route_pin_test.go` 的源码级断言钉住）。
//	写口只有一个：命令面 `zerg route pin|unpin`（人签的那一侧）。
package gateway

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/routepin"
)

// machineHeaderName —— 请求侧那一枚**零落盘**的一次性指定。
//
// 为什么用头而不是参数：头是**请求级**的（同一条连接上的下一次请求不会继承别人的意图），
// 且**不碰任何配置文件、不落任何盘** ⇒ 「偶尔的要求」天然是「偶尔的」。
const machineHeaderName = "X-Zerg-Machine"

// routePinStore —— 覆盖表的**只读**视图，按 (`mtime`, `size`) **热读**。
//
// 为什么要热读：钉/撒是命令面写的另一件进程 ⇒ 主控若只在启动时读一次，
// 「改完主控还得为每一次钉/撒重启」就成了新的绕路。热读让**钉/撒即时生效**
// （主控只要跑着带这道门的版本；那一次换件另说，见回执「需重启才生效」那一节）。
type routePinStore struct {
	mu    sync.Mutex
	path  string
	seen  bool
	mtime time.Time
	size  int64
	table *routepin.Table
}

// reloadIfChanged —— 盘面变过才重读（没变 ⇒ 直接用上次那份；变过 ⇒ 重读并**记下新形状**）。
// 读不出来（坏件/权限）⇒ 视图置空表（**不命中**，回默认择优）+ 一行日志：不静默、也不壮胆。
func (s *routePinStore) reloadIfChanged() *routepin.Table {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.path == "" {
		s.path = routepin.Path()
	}
	var mtime time.Time
	var size int64 = -1
	if st, err := os.Stat(s.path); err == nil {
		mtime, size = st.ModTime(), st.Size()
	}
	if s.seen && mtime.Equal(s.mtime) && size == s.size {
		return s.table
	}
	t, err := routepin.Load(s.path)
	s.seen, s.mtime, s.size = true, mtime, size
	if err != nil {
		// 读不到 ≠ 没有覆盖：不猜、不静默 —— 这一轮不命中，日志点名件与原因。
		s.table = &routepin.Table{ID: routepin.TableID}
		log.Printf("⚠️ 显式指定机器：覆盖表读不出来（%s）：%v ⇒ 这一轮不命中（回默认择优）", s.path, err)
		return s.table
	}
	s.table = t
	return s.table
}

// hostFor —— 该模型**当前生效**的显式指定机器（只读覆盖表那一档）。
func (s *routePinStore) hostFor(model string, now time.Time) (string, bool) {
	t := s.reloadIfChanged()
	p, ok := t.HostFor(model, now)
	if !ok {
		return "", false
	}
	return p.Machine, true
}

// pinStore —— Gateway 侧的懒建入口（**零值 Gateway 也能用** ⇒ 既有测试的构造面一字未动）。
func (g *Gateway) pinStore() *routePinStore {
	g.pinMu.Lock()
	defer g.pinMu.Unlock()
	if g.pinCache == nil {
		g.pinCache = &routePinStore{path: routepin.Path()}
	}
	return g.pinCache
}

// pinnedHostFor —— 覆盖表那一档 ⇒ (机器, 来源说明)。没命中 ⇒ ("", "")。
func (g *Gateway) pinnedHostFor(model string) (string, string) {
	s := g.pinStore()
	if h, ok := s.hostFor(model, time.Now()); ok {
		return h, "覆盖表 " + s.path
	}
	return "", ""
}

// machineHeaderOf —— 请求侧那一枚（零落盘的一次性路径）。没有 ⇒ 空串。
func machineHeaderOf(r *http.Request) string {
	if r == nil {
		return ""
	}
	return strings.TrimSpace(r.Header.Get(machineHeaderName))
}

// pickRouteForRequest —— 带**请求头**那一档的唯一入口。
//
// 头不给（99.9% 的请求）⇒ **逐字**走 `pickRoute`（于是「不钉时行为与今天逐字一致」这条铁律
// 在这一层就已经成立：多出来的只是一次字符串为空判断）。
// 头给了 ⇒ 命中即用（先于覆盖表 —— 「本次请求的意图」比「表里的长期意图」更具体）。
func (g *Gateway) pickRouteForRequest(model, headerMachine, sessionID, prompt string, required ...string) (*RouteResult, error) {
	if m := strings.TrimSpace(headerMachine); m != "" {
		return g.routeToPinned(model, m, "请求头 "+machineHeaderName, required)
	}
	// 默认规则那一档：**补一行落点日志**（2026-09-25）。
	//
	// 为什么要有：既有 `📍 routed to: host:port` 只记「落哪台」，不记「凭什么落的」——
	// 于是「同一会话在两台间间隔着跑」（缓存每次全丢 ⇒ 单次 10 分钟级延迟）这一类
	// 现象，事后只能靠缓存命中率反推，判不清是换机还是机器忙。这一行同时打出
	// **来源（默认规则）** 与 **会话号**，让「同一会话是否在跳机」一眼可证。
	// 只加日志、不动判决：本行之后的返回与改动前逐字一致。
	r, err := g.pickRoute(model, sessionID, prompt, required...)
	if err == nil && r != nil {
		sid := strings.TrimSpace(sessionID)
		if len(sid) > 8 {
			sid = sid[:8]
		}
		log.Printf("🎯 默认分配（来源 默认规则 · 无显式指定）：model=%s → %s（会话 %s）", model, r.Host, sid)
	}
	return r, err
}

// routeToPinned —— **命中即用**那一段（两条入口共用：请求头 / 覆盖表）。
//
// 步骤与取舍都写在这里，读的人不必去别处拼：
//
//	① 别名解析一次（与 `pickRoute` 同口径）—— 「钉的是标准名、请求用别名」不该落空；
//	② 目标机必须是该模型的候选 —— 不是 ⇒ fail-closed（不回默认择优）；
//	③ 能力硬门照走 —— 拦下 ⇒ fail-closed（**不豁免**）；
//	④ 组装 RouteResult（URL/端口取自 `fleetNode`，与既有的会话粘性那条同形）；
//	⑤ 熔断/心跳两枚信号**只记不判**（命中即用的代价，写在日志里让它看得见）。
func (g *Gateway) routeToPinned(model, machine, source string, required []string) (*RouteResult, error) {
	canon := model
	if g.config != nil {
		if _, ok := g.config.Models[canon]; !ok {
			if c, ok2 := g.config.Aliases[canon]; ok2 {
				canon = c
			}
		}
	}
	candidates, ok := g.config.Models[canon]
	if !ok {
		return nil, fmt.Errorf("显式指定机器（%s）：模型 %s 不在路由表里 ⇒ 不给结论也不换机器", source, model)
	}
	var cand *ModelCandidateView
	for i := range candidates {
		if candidates[i].Host == machine {
			cand = &ModelCandidateView{File: candidates[i].File, MemGb: int(candidates[i].MemGb)}
			break
		}
	}
	if cand == nil {
		return nil, fmt.Errorf("显式指定机器（%s）：模型 %s 在 %s 上没有候选 ⇒ **不悄悄回默认择优**（要么改指一台候选机，要么撒掉这条）",
			source, canon, machine)
	}
	if gerr := g.gateRoute(canon, machine, required); gerr != nil {
		return nil, fmt.Errorf("显式指定机器（%s）：模型 %s 在 %s 上被能力硬门拦下（**不豁免**）⇒ %w",
			source, canon, machine, gerr)
	}
	ip, port := g.fleetNode(machine)
	log.Printf("🔒 显式指定机器（来源 %s）：model=%s → %s（命中即用 —— 熔断/活性/轮询打分不参与判决）",
		source, canon, machine)
	if g.isTripped(machine) {
		log.Printf("⚠️ 显式指定机器：%s 当前**在熔断计数里**（T7）—— 命中即用照落，这一行只是把信号打出来", machine)
	}
	if g.snapshotFor(machine) == nil {
		log.Printf("⚠️ 显式指定机器：%s 当前**没有心跳快照** —— 命中即用照落（活性过滤不参与），这一行只是把信号打出来", machine)
	}
	return &RouteResult{
		Host:  machine,
		Port:  port,
		URL:   fmt.Sprintf("http://%s:%d/infer", ip, port),
		File:  cand.File,
		MemGB: cand.MemGb,
	}, nil
}

// ModelCandidateView —— 本件只从候选上取两格（文件 + 内存预算）。
//
// 为什么不直接持 `config.ModelCandidate`：本件对候选的必要信息就是这两格，
// 把整个结构体搬进来会让「本件依赖了候选的哪几格」变得不可见。
type ModelCandidateView struct {
	File  string
	MemGb int
}
