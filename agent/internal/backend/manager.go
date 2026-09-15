// Package backend 提供后端进程生命周期管理。
//
// 职责：
//   - 根据模型类型启动 llama-server 或 ds4-server
//   - 动态端口分配（9000-9999）
//   - 健康检查（交替探测 /health 和 /v1/models）
//   - 崩溃自愈（重试 3 次后熔断）
//   - 多模型驻留 + 五档裁决淘汰（§八 Q1 默认单槽 / Q2 五档；裁决逻辑在共享包 shared/resources，
//     本包只负责"把状态翻译成入参 + 执行裁决"——见 residency.go）
//   - 请求合并（同模型共享加载槽）
//   - 内存预算检查 + 只卸够腾空间（fail-closed：腾不出缺口就 507 拒装）
//   - 优雅停止（SIGTERM → 等 5s → SIGKILL）
package backend

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"errors"
	"github.com/Mr2109/zerg-swarm/agent/internal/modeladapter"
	"github.com/Mr2109/zerg-swarm/agent/internal/monitor"
	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
	"io"
)

// 状态机常量
const (
	StateIdle     = "idle"
	StateLoading  = "loading"
	StateReady    = "ready"
	StateCrashed  = "crashed"
	StateSleeping = "sleeping"
	// P2（设计 §7.7 修补 1）：显式状态「空窗计时中」与「draining」——定义在 p2_lifecycle.go。
	// 状态串：ready → 空窗计时中 → draining → stopped。
)

// subproc 单个模型的后端进程状态。
type subproc struct {
	proc     *exec.Cmd
	port     int
	model    string
	entry    *registry.ModelEntry
	state    string    // ready / loading / crashed / sleeping / idle_armed(空窗计时中) / draining
	failCnt  int       // 连续健康检查失败次数
	lastUsed time.Time // 最近使用时间（LRU）；P2 起兼作「最后一次活动时间戳」（§7.7 修补 2）
	reqCount int       // 活跃请求数（旧口径：InferForward 延迟减计数的观测面）
	// Unit 孵化单元的归属名（批 2，开关 ZERG_HATCH 开时由孵化器返回；开关关时**恒空**）。
	// 收卵判据就是它：非空 ⇒ 调 Hatcher.Collect（幂等），空 ⇒ 走既有进程句柄路径。
	// 它与 proc 是两种互斥形态：孵化路径没有本端进程句柄（proc 恒 nil），裸 exec 路径没有单元名。
	Unit string
	// enclosureVerified 孵化后的封闭性**实读核验**是否通过（§6.9 三态里的「通过」）。
	//
	// 只有拿到 /proc/<pid>/mountinfo 且判定通过才是 true；**「读不到」与「读到不符」都是 false**
	// ——两者靠 enclosureNote 区分，绝不把「没核到」当「核过了」。
	// 裸 exec 路径（孵化开关关）恒 false 且 note 空 = 本端从未声称过隔离。
	enclosureVerified bool
	// enclosureNote 核验留痕（一句话，进观测面 enclosure_note）：
	// 通过 = 结论 + 各项实测值；读不到 = 「未核验：<原因>」；不符 = 「核验不符：<哪几项>」。
	enclosureNote string
	// enginePID 孵化路径下**空间内引擎进程**的 pid（核验时实读到的那个；2026-09-15 缺陷 14）。
	//
	// 为什么需要它：孵化路径 sp.proc 恒 nil（引擎不是本端的子进程）⇒ 观测面原先拿不到 pid
	// ⇒ /eggs 缺 unit/gtt_gb，且 external_occupancy[] 把**本端自己孵的卵**误列为「外部占用者」。
	// 裸 exec 路径恒 0（那时用 sp.proc.Process.Pid）。
	enginePID int
	// inflight 在飞引用计数（P2，设计 §7.7 修补 3）——「在飞」的唯一真源：
	// 请求进入生成中 +1（acquireInflight）、完成/失败 −1（releaseInflight）。
	// 卸载/切换判据一律取它；引擎 /slots 只作交叉校验，不作为条件。
	inflight int
	// pinUntil Q5：显式 pin 的到期时刻（零值 = 未 pin）。无 TTL 的 pin 不允许——见 Manager.Pin。
	pinUntil time.Time
}

// loadWaiter 请求合并：同模型并发请求共享一个加载槽
type loadWaiter struct {
	done   chan struct{}
	result map[string]interface{}
	err    error
}

// Manager 后端管理器，线程安全。多模型驻留 + 五档裁决淘汰（Q1/Q2）。
type Manager struct {
	mu          sync.Mutex
	procs       map[string]*subproc    // model → subproc（多模型驻留）
	loading     map[string]*loadWaiter // model → 正在加载的等待组（请求合并）
	registry    *registry.Registry
	machine     string
	maxResident int // 驻留上限（<=0 视为默认单槽；见 EnvMaxResident / SetMaxResident）
	// idleTTL 空闲 TTL（《设计-资源管理器》§3.3(c) 触发①）：某驻留模型空闲（无在飞请求、
	// 非加载中、pin 未生效）持续超过它即卸载。缺省 = DefaultIdleTTL（300 秒，Mr2109 2026-09-13 拍板）；
	// ZERG_MODEL_TTL_S 覆盖，设 0 = 关闭；<=0 一律视为未启用。
	idleTTL time.Duration
	// reaperStop 后台 TTL 回收循环的停止信号（nil = 未启动）。读写都在 m.mu 下。
	reaperStop chan struct{}
	// stopHook（P2，测试注入）：stopSubproc 执行真正停进程动作前回调（nil = 无回调）。
	// 用途：验收「锁内置 draining ⇒ 出锁后才停进程」（§7.7 修补 3 的可观测面）。
	// 回调里如需读状态须自行加锁（回调发生在锁外）。
	stopHook func(sp *subproc)
	// waitQ 等待队列（P7：切换期到达的请求挂起在此，出队须重校验当前卵）。
	// ⚠ 锁序：任何持 m.mu 的路径都不得调用 WaitQ* 方法（见 waitqueue.go 文件头不变式）。
	waitQ *p2Queue
	// hatcher 孵化器实现（批 2：开关 ZERG_HATCH 开时用；nil ⇒ 默认 hatch.Hatcher{}）。
	// 抽成小接口只为测试注入假孵化器（见 hatch_path.go）——生产恒 nil，不改变任何现有行为。
	hatcher hatcher
}

// defaultReapInterval 是后台 TTL 回收循环的扫描间隔（只决定"多久查一次"，不是 TTL 本身）。
const defaultReapInterval = 30 * time.Second

// NewManager 创建后端管理器。驻留上限取自 ZERG_MAX_RESIDENT（默认单槽，Q1）；
// 空闲 TTL 取自 ZERG_MODEL_TTL_S（缺省 300 秒，见 DefaultIdleTTL；设 0 关闭）。TTL 启用时同时启动后台回收循环。
func NewManager(reg *registry.Registry, machine string) *Manager {
	m := &Manager{
		procs:       make(map[string]*subproc),
		loading:     make(map[string]*loadWaiter),
		registry:    reg,
		machine:     machine,
		maxResident: resolveMaxResident(),
		idleTTL:     resolveIdleTTL(),
	}
	// TTL 未启用（idleTTL<=0，例如显式设成 0）→ StartIdleReaper 直接返回，不产生后台 goroutine。
	m.StartIdleReaper(defaultReapInterval)
	// 启动 GC（§6.5「任何启动先把遗留卵清干净」）：上一轮子端留下的 zerg-* 卵单元不被 KillMode
	// 带走 ⇒ 会一直占着 GTT 且没有任何账本认领它们；启动时清掉（只清本子端名下的卵单元）。
	// 只在孵化开关开时执行（见 egg_gc.go 的两条口径：范围铁律 / 清不掉必须留痕）。
	gcLeftoverEggsAtStartup()
	return m
}

// Start 启动/复用模型对应的后端进程（多模型驻留）。
// - 模型已在驻留列表（ready）→ 直接复用（更新 lastUsed）
// - 模型正在加载（其他请求已触发）→ 等待同一加载槽（请求合并）
// - 不在驻留列表 → 加载新进程；驻留数超上限 → LRU 卸载
func (m *Manager) Start(modelName string) (map[string]interface{}, error) {
	// ⚠ 这里**不许**持 m.mu（2026-09-15 第一枚卵真机实测缺陷 1：自锁死）：
	// 本函数第一件事就是 m.RequestModel，而它自己会 m.mu.Lock()（p2_lifecycle.go RequestModel）
	// ——sync.Mutex 不可重入 ⇒ 持锁再调它 = 当场死锁：第一次 /load 就卡死，且同一把锁被
	// /status、心跳、巡检、事件循环争用 ⇒ 整个子端僵死（真机：/load 290s 无返回，
	// goroutine dump 停在 RequestModel:279）。
	// 锁改在下面「进入正常加载路径」处取（那条路径上的每条 return 都已配对解锁）。
	if needUnload, blocked := m.RequestModel(modelName); needUnload || blocked {
		// 异模型请求与当前卵冲突（§7.7 末条）：
		//   - needUnload=true：已锁内赢权置 draining——先停掉旧卵，再走正常孵化装新模型；
		//   - blocked=true：旧卵有在飞/pin/外部——不打断在飞，请求进等待队列（挂起，Q5）。
		if needUnload {
			// 本段自己取锁/放锁（Start 未持锁，见函数头注释）：只把「找出受害者」与
			// 「从账本摘掉」放在锁内，停进程在锁外（§7.7 修补 3 的锁序不变）。
			m.mu.Lock()
			var victimName string
			var victimSP *subproc
			for name, sp := range m.procs {
				if sp.state == StateDraining {
					victimName, victimSP = name, sp
					break
				}
			}
			var victimProc *exec.Cmd
			if victimSP != nil {
				victimProc = victimSP.proc
			}
			m.mu.Unlock()
			if victimSP != nil {
				// 真机缺陷 18（2026-09-15 复现于生产事故后半段 + 沙箱）：**先判能不能孵，再收卵**。
				// 原本顺序是「先收卵 → 再在 hatchGateLocked 里判声明/档案/资源」⇒ 一个注定被 507
				// 的异模型请求会白白把正在服务的卵收走、然后自己失败 ⇒ 服务归零。
				if rej := m.hatchPrecheck(modelName); rej != nil {
					m.restoreVictimLocked(victimName)
					log.Printf("[backend] ✗ 拒孵 %s（收卵前静态预检未过：卵未收、服务不受影响）: %v",
						modelName, rej["code"])
					return rej, nil
				}
				m.stopVictimProcess(victimProc, victimSP)
				m.mu.Lock()
				if cur, ok := m.procs[victimName]; ok && cur == victimSP {
					delete(m.procs, victimName)
				}
				m.mu.Unlock()
				log.Printf("[backend] 提前收卵完成: %s（异模型请求 %s 可孵化）", victimName, modelName)
				// 真机缺陷 17 附带：收卵后**有界等内存真正归还**再进内存闸门。
				// 症状：收掉一枚卵后立刻采样还是旧的（~35 GB 尚未归还/被观测到）⇒ 闸门判「不足」
				// ⇒ 卸了却没装上：卵没了、新模型 507、服务归零。
				waitMemoryReturn(10 * time.Second)
			}
			// 落到下方正常 Start（孵化）路径
		} else {
			// blocked：等待当前卵在飞清零（低频轮询重校验——禁 sleep 定时器直接动手；
			// 这里只等「可孵化」信号，收卵由巡检/后续请求完成）。上限 = 上行超时。
			deadline := time.Now().Add(120 * time.Second)
			for time.Now().Before(deadline) {
				time.Sleep(500 * time.Millisecond)
				if needU, blk := m.RequestModel(modelName); !blk {
					if needU {
						// 赢权成功——跳出去走孵化（递归一次，收卵段同上）
						// ⚠ 此处不得 Unlock：本函数**未**持 m.mu（自锁死修法见函数头注释）。
						return m.Start(modelName)
					}
					break
				}
			}
			// 等不到就继续往下走正常 Start（内部按红线拒装/拒孵，明确报错不硬来）
		}
	}

	// ── 进入正常加载路径：**此处**才取锁（本次改动前它在函数头上，见函数头注释）──────
	// 下面到 return 之间的每条路径都已配对解锁（unknown model / 已驻留复用 / 合并等待 /
	// 出锁后 doStart）。
	m.mu.Lock()

	// 查找模型配置
	entry, ok := m.registry.Get(modelName)
	if !ok {
		m.mu.Unlock()
		return errResponse(404, "unknown model", modelName), nil
	}

	// 已驻留且可用 → 直接复用（**不加载**）。
	//
	// 真机缺陷 17（2026-09-15 切生产后抓到）：
	//   ① 判据原本只认 StateReady，**不认 idle_armed**（空窗计时中的卵是活的、仍能服务）
	//      ⇒ 空窗中的卵被点名会掉进完整加载路径 ⇒ 内存闸门必然拒（内存正被它自己占着）
	//      ⇒ 507「insufficient memory」+ 可能触发收卵；
	//   ② 原本在这里 sp.inflight++（把「加载」记成「在飞」），而唯一的 releaseInflight 在
	//      InferForward 尾部（推理完成）⇒ 计数两头错位：加载 +1、推理 −1。
	//   现在：复用**不动** inflight —— inflight 的语义只有一种=正在被服务的推理请求数，
	//   获取/释放全在 InferForward（acquire 转发前、release 响应体读完时）。
	if sp, ok := m.procs[modelName]; ok && (sp.state == StateReady || sp.state == StateIdleArmed) {
		if sp.state == StateIdleArmed {
			log.Printf("[backend] 空窗取消（同模型 /load 复用）: model=%s 回 ready", modelName)
		}
		sp.lastUsed = time.Now()
		sp.reqCount++
		sp.state = StateReady
		m.mu.Unlock()
		log.Printf("[backend] 模型 %s 已驻留，直接复用 (port=%d)", modelName, sp.port)
		return okResponse(modelName, entry.Backend, sp.port), nil
	}

	// 请求合并：该模型正在加载 → 等待同一加载槽
	if w, ok := m.loading[modelName]; ok {
		m.mu.Unlock()
		log.Printf("[backend] 模型 %s 正在加载，等待共享加载槽 (请求合并)", modelName)
		<-w.done
		if w.err != nil {
			return nil, w.err
		}
		return w.result, nil
	}

	// 无驻留且无加载中 → 自己触发加载
	w := &loadWaiter{done: make(chan struct{})}
	m.loading[modelName] = w
	m.mu.Unlock()

	result, err := m.doStart(modelName, entry)

	m.mu.Lock()
	delete(m.loading, modelName)
	// ⚠ 顺序有讲究：**先写结果、再 close**。close 是等待方的 happens-before 边（<-w.done 之后才读
	// w.err/w.result）；原来的「先 close 后写」会让并发等待的请求与这两行写入竞争——真机并发
	// /load 时会读到半截结果，-race 也必报（本次并发 /load 用例抓到）。本改动不改变任何语义。
	w.result = result
	w.err = err
	close(w.done)
	m.mu.Unlock()
	return result, err
}

// doStart 实际执行加载流程（调用方需已持有加载槽）。
func (m *Manager) doStart(modelName string, entry *registry.ModelEntry) (map[string]interface{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// P1 接线（最先做）：适配器声明「本架构必须由非主线引擎承载」⇒ 卵声明必须带 cmd:
	// （专用 fork + 包装脚本含 env 处理），缺失即拒孵（502 语义），绝不静默退回主线
	// llama-server。放在一切裁决之前：声明校验属于「孵化前校验」（设计 §6.8.4），
	// 不该被内存采样/端口等因素抢先返回别的错误码。
	// 依据：设计-子端沙箱化-20260914 §1.2 / §4.7 / 附录 C·C1（fleet.yaml 无 cmd: 时第二台
	// 设备孵 K2 会静默退回主线 llama-server ⇒ 起不来或误链且不报错）。
	adpEarly := modeladapter.Dispatch(modelName)
	if nonMainline, ok := adpEarly.(modeladapter.NonMainlineEngine); ok && nonMainline.RequiresNonMainlineEngine() && entry.Cmd == "" {
		log.Printf("[backend] ✗ 拒孵 %s：该架构必须由非主线引擎承载（专用 fork + 包装脚本），卵声明缺少 cmd: 字段", modelName)
		return errResponse(502, "engine implementation missing",
			fmt.Sprintf("model %s requires a non-mainline engine (cmd: field in the egg manifest); refusing to silently fall back to the mainline llama-server", modelName)), nil
	}

	// M10「身份一致直接复用基线服务」已随 P4 退场清理删除（设计 §10.2 F1 / 附录 C·C2）：
	// 卵之外无引擎（§1.3）——子端之外的推理进程不是后端，不再登记复用。

	// 批 2 开关（纯环境读，无状态、无锁）：本函数里所有「孵化专属」动作都看它。
	// 开关**关** ⇒ 下面每一段孵化专属代码都跳过，doStart 的代码路径与孵化器落地前逐字一致。
	hatchMode := hatchEnabled()

	// P1 接线（§6.8.4「先判格式，再算账」）：**卵声明校验排在一切算账与裁决之前** ——
	// 先判格式（认不认得这枚卵的声明），再算账（淘汰、内存预检、双闸门 GTT/内存账）。
	// 与上面「非主线引擎必须带 cmd:」同一处口径：声明类错误不该被内存采样 / 端口 / 账读不到
	// 抢先返回别的错误码（拒因必须可归因），也不该为了一枚格式就不对的卵去驱逐在孵的卵。
	//
	// 接线理由（2026-09-15 第一枚卵真机实测缺陷 8）：registry.ValidateEggDeclaration /
	// ValidateEgg 此前**在生产路径无任何调用点**（全仓只有定义）⇒ schema_version / env_req
	// 必填项校验实际不生效（env_req 缺 lib_paths（设计上是 Fatal）也照孵）。
	// 开关关时不执行：本子端不孵任何卵，卵声明的孵化语义无从生效（且离线路径承诺逐字不变）。
	if hatchMode {
		dec := registry.ValidateEggDeclaration(modelName, entry)
		if !dec.OK() {
			detail := dec.ErrorString()
			log.Printf("[backend] ✗ 拒孵 %s：卵声明校验不过（§6.8.4 先判格式、再算账）：%s", modelName, detail)
			return errResponse(502, "egg declaration rejected", detail), nil
		}
		// Warnings 照孵，但**必须报出去**（绝不静默按缺省值跑，§6.9 第 3 条）。
		if w := dec.WarningString(); w != "" {
			log.Printf("[backend] ⚠ 卵声明告警（照孵，但必须报出去）%s：%s", modelName, w)
		}
	}

	// 驻留超限 → 五档裁决淘汰（Q1 默认单槽 / Q2 五档）
	m.evictIfNeededLocked()

	// P1：跨服务类型强制清场（llama xor ds4）——受管侧保证「同一时间只有一种服务在跑」
	// （设计 §7 S2）。异类且可动作 ⇒ 卸下；异类被红线挡住（在飞/pin/加载中）⇒ 拒装，
	// 绝不硬来。未托管的手工服务不在此列（走 baseline/借用机制，见设计 §9）。
	kind := serviceKind(entry)
	if ev, blocked := m.evictOtherKindsLocked(modelName, kind); len(ev) > 0 || len(blocked) > 0 {
		log.Printf("[backend] 服务类型互斥(%s): 卸下=%v 红线挡住=%v", kind, ev, blocked)
		if len(blocked) > 0 {
			return errResponse(507, "service-kind conflict",
				fmt.Sprintf("另一类服务(%s)仍有受保护驻留: %v；本机同一时间只允许一种推理服务在跑",
					kind, blocked)), nil
		}
	}

	// 内存预算检查（T3 预检，借鉴 llama_cpp_router willModelFit）——fail-closed
	if memRequired := entry.MemGB; memRequired > 0 {
		avail := monitor.DefaultSampler.MemAvailableGb()
		ok, have, need := m.ensureMemoryForLocked(memRequired, avail)
		if !ok {
			// 腾不出缺口 → 拒装（507 语义不变）：不许赌"应该能跑"。
			return m.rejectInsufficientMemory(have, need), nil
		}
		log.Printf("[backend] 内存预算检查通过: avail=%.1fGB, required=%.1fGB", have, need)
	}

	// ── 批 2：开关开 ⇒ 先判实测档案、再过双闸门（§8.4 / §6.8.4「先判格式，再算账」）─────────
	// 声明校验（格式侧）已在函数开头接好；此处是**算账侧**（实测档案 + 两账）。
	// 开关**关** ⇒ 这一段整体不执行（hatchMode=false），doStart 的代码路径与孵化器落地前逐字一致。
	// 拒孵一律在**建 subproc / 分配端口之前**返回：不起任何单元、不登记任何驻留（账本干净）。
	var hatchProfile monitor.EggProfile
	if hatchMode {
		prof, reject := m.hatchGateLocked(modelName, entry)
		if reject != nil {
			return reject, nil
		}
		hatchProfile = prof
	}

	// 设置 loading 状态
	sp := &subproc{
		model:    modelName,
		entry:    entry,
		state:    StateLoading,
		lastUsed: time.Now(),
	}
	m.procs[modelName] = sp

	// 动态端口分配
	port := m.findFreePort()
	if port == 0 {
		sp.state = StateCrashed
		return errResponse(500, "no free port", ""), nil
	}
	sp.port = port

	// 批 2：开关开 ⇒ 走孵化器（起单元 + 既有就绪判据 + 封闭性核验）；开关关 ⇒ 走下方既有裸 exec
	// 路径（一行不变）。两者共用同一份参数构造（buildEngineArgv）。
	if hatchMode {
		return m.hatchStartLocked(modelName, entry, sp, port, hatchProfile)
	}

	// 构建启动命令（模型适配层：按模型名选适配器，管理启动参数/工具风格/重提示）
	// （P1 的「非主线引擎必须带 cmd:」守卫在 doStart 最前面，见函数开头。
	//  批 1：构造整段抽成 buildEngineArgv —— 孵化路径（hatch_spec.go）要用**同一份**参数，
	//  两处各写一遍必然漂移；此处逐字保持原逻辑，行为不变。）
	cmdPath, execArgs := buildEngineArgv(modelName, entry, port)

	cmd := exec.Command(cmdPath, execArgs...)
	sp.proc = cmd
	log.Printf("[backend] spawn 命令: %s %v", cmdPath, execArgs)

	stderrPipe, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		sp.state = StateCrashed
		return errResponse(500, "failed to start backend", err.Error()), nil
	}
	if stderrPipe != nil {
		go func() {
			buf := make([]byte, 4096)
			for {
				n, err := stderrPipe.Read(buf)
				if n > 0 {
					log.Printf("[backend] 后端stderr: %s", string(buf[:n]))
				}
				if err != nil {
					break
				}
			}
		}()
	}

	pid := cmd.Process.Pid
	log.Printf("[backend] 启动 %s: pid=%d, port=%d, model=%s", entry.Backend, pid, port, modelName)

	// 等待健康检查通过
	if err := m.waitForReady(sp); err != nil {
		log.Printf("[backend] 健康检查失败: %v", err)
		m.stopSubproc(sp)
		sp.state = StateCrashed
		return errResponse(500, "health check failed", err.Error()), nil
	}

	// M3：**功能预检** —— "加载成功 ≠ 可用"（今天 502 的直接成因）。
	// 引擎自己还有守卫：`rocm prefill failed` 正是在"装好了"之后才发生。
	// 判据必须是一次**真实生成**（max_tokens=1）：只看 /health 只能证明端口活。
	if probeErr := probeInference(port, modelName, probeTimeout()); probeErr != nil {
		log.Printf("[backend] 功能预检未通过: model=%s port=%d: %v", modelName, port, probeErr)
		// 设计 §11 M3 ③：明确报错 + **归还**（停掉本端起的进程），绝不对外声称已就绪。
		m.stopSubproc(sp)
		sp.state = StateCrashed
		delete(m.procs, modelName)
		return errResponse(502, "model unusable after load (functional probe failed)", probeErr.Error()), nil
	}
	log.Printf("[backend] 功能预检通过: model=%s port=%d", modelName, port)

	// 就绪
	sp.failCnt = 0
	sp.state = StateReady
	log.Printf("[backend] 后端就绪: port=%d, model=%s", port, modelName)
	return okResponse(modelName, entry.Backend, port), nil
}

// Stop 停止所有后端进程。
func (m *Manager) Stop() map[string]interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, sp := range m.procs {
		// 批 2：孵化路径的卵没有本端进程句柄（sp.proc 恒 nil）⇒ 凭 sp.Unit 非空判定它需要收卵。
		// 开关关时 sp.Unit 恒空（没有任何代码写它）⇒ 判据与原来逐字等价。
		if (sp.proc != nil && sp.proc.ProcessState == nil) || sp.Unit != "" {
			log.Printf("[backend] 停止进程: model=%s", name)
			m.stopSubproc(sp)
		}
		delete(m.procs, name)
	}
	return okResponse("", "", 0)
}

// State 返回主状态（有 ready 进程即 ready，否则 idle）。
func (m *Manager) State() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, sp := range m.procs {
		if sp.state == StateReady {
			return StateReady
		}
		if sp.state == StateLoading {
			return StateLoading
		}
	}
	return StateIdle
}

// CurrentModel 返回当前加载的模型名（驻留列表中最新的）。
func (m *Manager) CurrentModel() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var newest string
	var newestTime time.Time
	for _, sp := range m.procs {
		if sp.state == StateReady {
			if newest == "" || sp.lastUsed.After(newestTime) {
				newest = sp.model
				newestTime = sp.lastUsed
			}
		}
	}
	return newest
}

// CurrentBackend 返回主后端的类型。
func (m *Manager) CurrentBackend() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, sp := range m.procs {
		if sp.state == StateReady && sp.entry != nil {
			return sp.entry.Backend
		}
	}
	return ""
}

// CurrentPort 返回主后端的端口。
func (m *Manager) CurrentPort() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, sp := range m.procs {
		if sp.state == StateReady {
			return sp.port
		}
	}
	return 0
}

// IsHealthy 检查后端是否健康（任一驻留模型健康即可）。
func (m *Manager) IsHealthy() bool {
	m.mu.Lock()
	var candidates []*subproc
	for _, sp := range m.procs {
		if sp.state == StateReady && sp.port > 0 {
			candidates = append(candidates, sp)
		}
	}
	m.mu.Unlock()

	for _, sp := range candidates {
		if m.healthCheck(sp) {
			return true
		}
	}
	return false
}

// IsModelHealthy 检查指定模型是否健康。
func (m *Manager) IsModelHealthy(model string) bool {
	m.mu.Lock()
	sp, ok := m.procs[model]
	m.mu.Unlock()
	if !ok || sp.state != StateReady {
		return false
	}
	return m.healthCheck(sp)
}

// InferForward 转发推理请求到指定模型的后端。
// 若后端处于 crashed 状态，自动重置熔断，由上层 Load 流程重新启动。
// v2.5.6 治本（2026-08-28——x3 幽灵请求）: ctx 参数——客户端断开时取消后端请求——释放单槽
func (m *Manager) InferForward(ctx context.Context, model string, path string, body []byte) (*http.Response, error) {
	m.mu.Lock()
	sp, ok := m.procs[model]
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("模型 %s 未驻留", model)
	}
	if sp.state == StateCrashed {
		log.Printf("[backend] 检测到 crashed，自动重置熔断，准备重启后端")
		sp.failCnt = 0
		sp.state = StateLoading
	}
	// idle_armed（空窗计时中）也是活的、能服务——只是"没人用"而已（§7.1 补记）。
	if (sp.state != StateReady && sp.state != StateIdleArmed) || sp.port == 0 {
		state := sp.state
		m.mu.Unlock()
		return nil, fmt.Errorf("后端未就绪 (state=%s)", state)
	}
	port := sp.port
	m.mu.Unlock()

	// ── 在飞记账（真机缺陷 17 核心修复，2026-09-15）─────────────────────────
	// 语义（唯一一种）：inflight = 「正在被服务的推理请求数」。
	//   acquire：转发**之前**；release：响应体**读完并 Close** 时（转发返回 ≠ 推理完成，
	//   流式尤其如此——所以把 release 挂在 body.Close 上）。
	// 为什么必须补：本函数原来只在尾部 releaseInflight（−1），而 acquire 侧**零调用点**
	//   ⇒ 生成进行中 inflight==0 ⇒ RequestModel 的「异模型请求：有在飞就先等生成跑完」
	//   判据永不成立 ⇒ 异模型 /load 会当场收卵，把正在出字的请求砍断（客户端 500 EOF）。
	if !m.acquireInflight(model) {
		return nil, fmt.Errorf("后端不可服务 (state=%s)", sp.state)
	}
	// relOnce：释放恰好一次（defer 兜底 + body.Close 两条路都走这个幂等闭包）。
	var relOnce sync.Once
	release := func() { relOnce.Do(func() { m.releaseInflight(model) }) }
	handedOff := false
	defer func() {
		if !handedOff {
			release() // 未把响应体交出去（构造/转发失败）⇒ 在此释放，绝不留悬挂计数
		}
	}()

	// 请求前健康检查
	if !m.healthCheck(sp) {
		m.mu.Lock()
		sp.failCnt++
		if sp.failCnt >= 3 {
			log.Printf("[backend] 熔断触发 (连续失败 %d 次)", sp.failCnt)
			m.stopSubproc(sp)
			sp.state = StateCrashed
			m.mu.Unlock()
			return nil, fmt.Errorf("后端已熔断 (连续失败 %d 次)", sp.failCnt)
		}
		log.Printf("[backend] 健康检查失败 (%d/3)，重试中...", sp.failCnt)
		m.mu.Unlock()
		time.Sleep(500 * time.Millisecond)
		if !m.healthCheck(sp) {
			return nil, fmt.Errorf("健康检查重试仍失败")
		}
		m.mu.Lock()
		sp.failCnt = 0
		m.mu.Unlock()
	}

	url := fmt.Sprintf("http://127.0.0.1:%d%s", port, path)
	// v2.5.6 治本: 用客户端 context 构造请求——断开自动取消（不再等 llama-server 生成完——释放单槽）
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("构造请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 5 * time.Minute,
		// 治本（2026-08-12）：禁 keep-alive——llama-server 连接空闲被关，
		// agent 复用断连接 → 长响应读断（IncompleteRead——网关 EOF 根因链）
		Transport: &http.Transport{
			DisableKeepAlives: true,
			// v2.5.4.9 首 token 超时（90s——llama-server 卡死检测——active 释放）
			// 知识库经验: 大请求(含tools)→ornith 38-60s 推理(正常)——60s 误杀——调 90s
			// 真卡死: 90s 无响应头 → 返回错误 → active 释放 → 主控 failover
			ResponseHeaderTimeout: 90 * time.Second,
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		if !m.healthCheck(sp) {
			m.mu.Lock()
			sp.failCnt++
			if sp.failCnt >= 3 {
				m.stopSubproc(sp)
				sp.state = StateCrashed
			}
			m.mu.Unlock()
		}
		return nil, fmt.Errorf("转发到后端失败: %w", err)
	}

	// 请求成功，更新 lastUsed + 重置失败计数
	m.mu.Lock()
	sp.failCnt = 0
	sp.lastUsed = time.Now()
	sp.reqCount++
	m.mu.Unlock()
	// 在飞 −1 不在这里：交给响应体 Close（见函数头注释）——转发返回只是"头回来了"，
	// 客户端把 body 读干净才算本次推理结束；归零时 releaseInflight 转「空窗计时中」。
	resp.Body = &releaseOnCloseBody{ReadCloser: resp.Body, release: release}
	handedOff = true
	return resp, nil
}

// hatchPrecheck 孵前静态预检（**只读**：不淘汰、不收卵、不改任何状态）。
//
// 真机缺陷 18（2026-09-15，生产事故后半段与沙箱各复现一次）：`Start` 原本先把当前卵收掉，
// 之后才在 hatchGateLocked 里判「新模型有没有档案 / 资源够不够」⇒ 一个注定被 507 的请求
// 会白白把正在服务的卵收走、然后自己失败 ⇒ 净结果服务归零。
// 现在：动手收卵**之前**先跑同一套静态判据（声明 → 档案 → 资源 dry-run），不过就原样退回。
//
// 资源 dry-run 口径：把「将被收掉的那枚卵」的占用**算作可用**（模拟收掉之后），
// 这样正常的「换卵」流程不受影响；只有「收了也不够 / 不合规」才会在这里被挡下。
func (m *Manager) hatchPrecheck(modelName string) map[string]interface{} {
	if !hatchEnabled() {
		return nil // 未开孵化：这条静态判据不适用（行为与开关关时逐字一致）
	}
	entry, ok := m.registry.Get(modelName)
	if !ok {
		return errResponse(404, "unknown model", modelName)
	}
	if dec := registry.ValidateEggDeclaration(modelName, entry); !dec.OK() {
		return errResponse(502, "egg declaration rejected", dec.ErrorString())
	}
	profilePath := monitor.EggProfilePath(strings.TrimSpace(entry.EggName()))
	if profilePath == "" {
		return errResponse(507, "no measured profile", "卵名拿不到，实测档案路径算不出来")
	}
	prof, err := monitor.LoadEggProfile(profilePath)
	if err != nil {
		return errResponse(507, "no measured profile",
			fmt.Sprintf("无有效实测档案，拒孵（标定铁律 §8.4：闸门与预算只读实测档案，卵声明里的估值不参与）（档案=%s：%v）", profilePath, err))
	}
	gttAvail, memAvail, acctErr := hatchGateRead()
	if acctErr != nil {
		return errResponse(507, "hatch gate unreadable",
			fmt.Sprintf("闸门两账读不到，拒孵（fail-closed：算不出就不装）：%v", acctErr))
	}
	// dry-run：把「将被收掉的卵」（draining）的占用加回来
	m.mu.Lock()
	for _, sp := range m.procs {
		if sp.state != StateDraining || sp.entry == nil {
			continue
		}
		if vp, verr := monitor.LoadEggProfile(monitor.EggProfilePath(strings.TrimSpace(sp.entry.EggName()))); verr == nil {
			gttAvail += vp.PeakGttGb
			memAvail += vp.PeakMemGb
		} else if sp.entry.MemGB > 0 {
			gttAvail += sp.entry.MemGB
			memAvail += sp.entry.MemGB
		}
	}
	m.mu.Unlock()
	if res := monitor.CanHatchWithProfile(true, gttAvail, memAvail, prof); !res.Ok {
		return errResponse(507, "insufficient memory",
			fmt.Sprintf("%s（dry-run：把待收卵的占用算作可用后，GTT 可用 %.1f GB / 内存可用 %.1f GB；档案 peak_gtt=%.1f GB peak_mem=%.1f GB）",
				res.Reason, gttAvail, memAvail, prof.PeakGttGb, prof.PeakMemGb))
	}
	return nil
}

// restoreVictimLocked 静态预检挡下时，把「已置 draining 但尚未动手」的卵放回 ready——
// 没停进程、没清账本 ⇒ 服务不受影响（只是白点了一次名，指针回零）。
func (m *Manager) restoreVictimLocked(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if sp, ok := m.procs[name]; ok && sp.state == StateDraining {
		sp.state = StateReady
		sp.lastUsed = time.Now()
		log.Printf("[backend] 卵已放回（收卵前预检未过）: %s 回 ready", name)
	}
}

// releaseOnCloseBody 把「在飞 −1」挂在响应体 Close 上。
//
// 非流式：调用方 io.ReadAll 后 Close ⇒ 读干净即释放；流式：边读边写客户端，Close 即释放。
// ⚠ release 自身必须是幂等的（本类型**不**再套一层 Once——否则同一把 Once 嵌套自锁：
// 首版就踩了，见缺陷 17 的用例 TestDefect17_InferForwardMaintainsInflight）。
type releaseOnCloseBody struct {
	io.ReadCloser
	release func()
}

func (b *releaseOnCloseBody) Close() error {
	err := b.ReadCloser.Close()
	b.release()
	return err
}

// IsBackendBusy 判断转发失败是否属于「后端忙/在忙别的」这一类（真机缺陷 17 附带）。
//
// 语义：单槽引擎被占、响应头超时、连接被引擎主动断开 ⇒ 应当**稍后重试**，
// 而不是「后端坏了」。调用方据此回 503 + retry_after（而不是 500/502 让上游当故障换机）。
func IsBackendBusy(err error) bool {
	if err == nil {
		return false
	}
	var nerr net.Error
	if errors.As(err, &nerr) && nerr.Timeout() {
		return true
	}
	s := err.Error()
	return strings.Contains(s, "timeout awaiting response headers") ||
		strings.Contains(s, "connection reset by peer") ||
		strings.HasSuffix(s, ": EOF")
}

// waitMemoryReturn 收卵后**有界**等可用内存真正归还（判据：连续两次采样不再上升即稳定）。
// 上限 max；调用方不应持锁（本函数会 sleep）。
func waitMemoryReturn(max time.Duration) {
	prev := monitor.DefaultSampler.MemAvailableGb()
	flat := 0
	deadline := time.Now().Add(max)
	for time.Now().Before(deadline) {
		time.Sleep(300 * time.Millisecond)
		cur := monitor.DefaultSampler.MemAvailableGb()
		if cur > prev+0.05 {
			flat = 0
			prev = cur
			continue
		}
		flat++
		if flat >= 2 {
			return
		}
	}
}

// RegistryNames 返回注册表模型名列表（心跳上报用）。
func (m *Manager) RegistryNames() []string {
	return m.registry.Names()
}

// BackendRssGb 返回后端进程总内存占用（GB，心跳上报用）。
func (m *Manager) BackendRssGb() float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	var total float64
	for _, sp := range m.procs {
		if sp.proc != nil && sp.proc.Process != nil {
			total += readMacOSRss(sp.proc.Process.Pid)
		}
	}
	return total
}

// ensureMemoryForLocked 内存预检（fail-closed，调用方需持锁）。
//
// 口径：新模型需要 memRequiredGb*1.1；实测可用 availGb 不够时，先按五档裁决**只卸够**
// 缺口（avail - need），仍不够就返回 ok=false（调用方回 507 —— 估不出/腾不出就不装）。
// 返回 (ok, 腾退后的可用量, 需求量)。
//
// availGb 由调用方传入（doStart 传 monitor 实测值）——这样"够不够"的判定可被测试注入，
// 不必去动真机内存。
func (m *Manager) ensureMemoryForLocked(memRequiredGb, availGb float64) (bool, float64, float64) {
	need := memRequiredGb * 1.1
	if availGb >= need {
		return true, availGb, need
	}
	// 只卸够：缺口 = need - avail（不是模型总需求——旧实现传总量，等于多卸）
	freed := m.evictForMemoryLocked(need - availGb)
	have := availGb + freed
	if have < need && freed > 0 {
		// 腾退过但仍不够：归还可能还没被观测到（真机缺陷 17 附带）——短重采样再判一次。
		// 有界（≤1.2s），调用方持锁但时长可控。
		for i := 0; i < 4 && have < need; i++ {
			time.Sleep(300 * time.Millisecond)
			have = monitor.DefaultSampler.MemAvailableGb()
		}
	}
	return have >= need, have, need
}

// rejectInsufficientMemory 内存不足的拒装响应（507 insufficient memory——语义与既有实现一致）。
func (m *Manager) rejectInsufficientMemory(have, need float64) map[string]interface{} {
	return errResponse(507, "insufficient memory", fmt.Sprintf("available: %.1f GB, required: %.1f GB", have, need))
}

// evictIfNeededLocked 驻留超限时按五档裁决淘汰（调用方需持锁）。
//
// 上限 = maxResident（默认 1，Q1；配置项 ZERG_MAX_RESIDENT）。淘汰顺序不再只是 LRU，
// 而是共享裁决的五档：① 崩溃/僵尸（本端 procs 里只有我们自己起的进程，未托管者根本不在其中）
// → ③ 空闲中 LRU 最旧、同档权重更大者先（腾得多）→ ⑤ 被别处等待/加载中者最后；
// 在飞请求者（红线①）与 pin 未到期者（红线③）绝不进入列表。
func (m *Manager) evictIfNeededLocked() {
	limit := m.residentLimitLocked()
	if len(m.procs) < limit {
		return
	}
	// 要腾出的槽位数：让新模型进来后不超过上限
	needSlots := len(m.procs) - limit + 1
	// 准入过滤后的可动作清单（在飞/未托管/pin 未到期/加载中已被拦住）
	plan := m.rankActionableLocked()
	if len(plan) == 0 {
		// 无可驱逐项（全在飞 / 全 pin 未到期 / 全加载中）：宁可超限也不杀正在用的模型——
		// 内存预检与 507 兜底（fail-closed），并如实记日志。
		log.Printf("[backend] 驻留超限但无可驱逐项（在飞/pin/加载中受保护）: resident=%d limit=%d", len(m.procs), limit)
		return
	}
	if len(plan) < needSlots {
		log.Printf("[backend] 可驱逐 %d 项 < 需腾 %d 槽：只腾能腾的（受保护项不动）", len(plan), needSlots)
		needSlots = len(plan)
	}
	for _, e := range plan[:needSlots] {
		sp, ok := m.procs[e.Alias]
		if !ok {
			continue
		}
		m.evictSubprocLocked(e.Alias, sp, fmt.Sprintf("五档淘汰(tier=%d,%s)", e.Tier, e.Reason))
	}
}

// evictForMemoryLocked 只卸够：按五档裁决腾出缺口 needGB，返回实际腾出的 GB。
//
// needGB 语义 = "还差多少内存"（不是模型总需求）——调用方传 need - 当前可用，
// 这样"卸一个就够"才成立（旧实现传总量，等于多卸）。
// 在飞请求者、pin 未到期者、加载中者、未托管进程一律不进计划（红线①②③）。
func (m *Manager) evictForMemoryLocked(needGB float64) float64 {
	if needGB <= 0 {
		return 0
	}
	victims := m.evictVictimsLocked(needGB)
	var freed float64
	for _, name := range victims {
		sp, ok := m.procs[name]
		if !ok {
			continue
		}
		freed += m.evictSubprocLocked(name, sp, "内存腾退(只卸够)")
	}
	return freed
}

// 内部：停止进程（SIGTERM → 等 5s → SIGKILL）
func (m *Manager) stopSubproc(sp *subproc) {
	if sp == nil {
		return
	}
	// P2（§7.7 修补 3）：真正动手前的观测钩子——此刻调用方应已在锁内置 draining 并出锁。
	if m.stopHook != nil {
		m.stopHook(sp)
	}
	// 批 2：孵化路径收卵 —— sp.Unit 非空 ⇒ 改调孵化器 Collect（幂等，systemctl --user stop），
	// 没有本端进程句柄可发信号。开关关时 sp.Unit 恒空 ⇒ 这段与既有句柄路径互不干扰。
	if sp.Unit != "" {
		m.collectUnit(sp.Unit)
		sp.proc = nil
		sp.port = 0
		return
	}
	if sp.proc == nil || sp.proc.Process == nil {
		return
	}

	if err := sp.proc.Process.Signal(syscall.SIGTERM); err != nil {
		log.Printf("[backend] SIGTERM 发送失败: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- sp.proc.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			log.Printf("[backend] 进程退出: %v", err)
		} else {
			log.Printf("[backend] 进程已优雅退出")
		}
	case <-time.After(5 * time.Second):
		log.Printf("[backend] 等待退出超时 (5s)，发送 SIGKILL")
		sp.proc.Process.Kill()
		sp.proc.Wait()
	}

	sp.proc = nil
	sp.port = 0
}

// 内部：健康检查（交替探测 /health 和 /v1/models）
func (m *Manager) healthCheck(sp *subproc) bool {
	if sp == nil || sp.port == 0 {
		return false
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/health", sp.port)
	resp, err := http.Get(url)
	if err == nil {
		resp.Body.Close()
		if resp.StatusCode == 200 {
			return true
		}
	}

	url = fmt.Sprintf("http://127.0.0.1:%d/v1/models", sp.port)
	resp, err = http.Get(url)
	if err == nil {
		resp.Body.Close()
		if resp.StatusCode == 200 {
			return true
		}
	}
	return false
}

// 内部：等待后端就绪（最多 120 秒）
//
// 三条出口（**不许**把 120s 硬等改成更短的硬等——那只是把「慢」当「死」，慢启动的正确卵会被误杀）：
//
//	① 健康检查通过 ⇒ 就绪；
//	② 裸 exec 路径：本端进程已退出（sp.proc 有句柄）⇒ 明确报错；
//	③ 孵化路径：单元已不在运行 ⇒ 读 systemd 的 ActiveState/SubState/Result/ExecMainStatus
//	   给明确报错。③ 是 2026-09-15 第一枚卵真机实测缺陷 9 的修法：孵化路径下 sp.proc 恒 nil
//	   （引擎不是子端的子进程）⇒ 原先两条出口都不成立，单元 1 秒死也只能**干等满 120s**
//	   （真机实测 120.16s）才回「等待后端就绪超时」，失败原因不可归因。
//	   探不到单元状态（非 Linux / systemctl 不在）⇒ **不下结论**，继续按 ① 等（与既有行为一致）。
func (m *Manager) waitForReady(sp *subproc) error {
	const timeout = 120 * time.Second
	deadline := time.Now().Add(timeout)
	interval := 2 * time.Second
	probeLogged := false

	for time.Now().Before(deadline) {
		if m.healthCheck(sp) {
			return nil
		}
		if sp.proc != nil && sp.proc.ProcessState != nil {
			return fmt.Errorf("后端进程已退出")
		}
		if sp.Unit != "" {
			st, err := unitStateProbe(sp.Unit)
			switch {
			case err != nil:
				// §6.9 同一精神：读不到就不下结论（既不说「死了」，也不说「好了」）。
				if !probeLogged {
					log.Printf("[backend] ⚠ 单元状态探不到，按「还没就绪」继续等: unit=%s: %v", sp.Unit, err)
					probeLogged = true
				}
			default:
				if dead, why := st.dead(); dead {
					detail := why
					if st.JournalTail != "" {
						detail += "；单元日志尾部：" + st.JournalTail
					}
					return fmt.Errorf("孵化单元已退出（%s）", detail)
				}
			}
		}
		time.Sleep(interval)
	}
	return fmt.Errorf("等待后端就绪超时 (%s)", timeout)
}

// EnvPortPool 子端专用端口区间（如 "9400-9999"）。未设置时用默认 9000-9999。
const EnvPortPool = "ZERG_PORT_POOL"

// findFreePort 分配空闲端口。
//
// 可用 ZERG_PORT_POOL="9400-9999" 把子端完全隔离到专用区间。
// （baseline 声明端口的那道排除已随 P4 退场清理删除——卵之外无引擎后不存在
// "手工服务的坑"，附录 C·C8。）
func (m *Manager) findFreePort() int {
	lo, hi := 9000, 9999
	if v := strings.TrimSpace(os.Getenv(EnvPortPool)); v != "" {
		if a, b, ok := strings.Cut(v, "-"); ok {
			if n1, err1 := strconv.Atoi(strings.TrimSpace(a)); err1 == nil {
				if n2, err2 := strconv.Atoi(strings.TrimSpace(b)); err2 == nil && n1 > 0 && n2 <= 65535 && n1 <= n2 {
					lo, hi = n1, n2
				}
			}
		}
	}
	for port := lo; port <= hi; port++ {
		// 跳过已被本管理器其他模型占用的端口
		taken := false
		for _, sp := range m.procs {
			if sp.port == port {
				taken = true
				break
			}
		}
		if taken {
			continue
		}
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			continue
		}
		ln.Close()
		return port
	}
	return 0
}

// 内部：设置状态（旧接口保留，用于兼容）
func (m *Manager) setState(s string) {
	// no-op：状态现在由各 subproc 管理
}

// detectLlamaServerPath 探测 llama-server 可执行文件路径。
func detectLlamaServerPath() string {
	if path, err := exec.LookPath("llama-server"); err == nil {
		return path
	}
	candidates := []string{
		"/opt/homebrew/bin/llama-server", // macOS homebrew
		"/usr/local/bin/llama-server",    // Linux
		// X3——2026-08-30 统一单一库（build-hip-flash：支持 qwen4exp/PLE——旧 build-hip 已退役）
		"/home/g01/llama.cpp-src/build-hip-flash/bin/llama-server",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return "llama-server"
}

// 内部：构造响应
func okResponse(model, backend string, port int) map[string]interface{} {
	return map[string]interface{}{
		"ok":      true,
		"model":   model,
		"backend": backend,
		"port":    port,
	}
}

func errResponse(status int, code, msg string) map[string]interface{} {
	return map[string]interface{}{
		"ok":     false,
		"status": status,
		"code":   code,
		"error":  msg,
	}
}

// 兼容旧接口：readMacOSRss 用 ps -o rss=<pid> 读 RSS（kB）转 GB。
func readMacOSRss(pid int) float64 {
	out, err := exec.Command("/bin/ps", "-o", "rss=", "-p", fmt.Sprintf("%d", pid)).Output()
	if err != nil {
		return 0
	}
	kb, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0
	}
	return kb / 1024 / 1024
}
