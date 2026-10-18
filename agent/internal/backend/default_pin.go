// default_pin.go —— 默认模型上钉与「禁止挤」（缺口 GAP-20261019-01）。
//
// # 缺口（真机铁证）
//
// `Qwen3.8-Flash-Next` 是对话面的默认模型（`core/internal/api/chat_handlers.go:242`），
// 而子端单槽（`ZERG_MAX_RESIDENT` 缺省 1）。另一个装载请求（最危险的是**一次普通
// `/infer` 触发的自动装载**）会把它挤掉：`zerg ask` 仍出正文，但回答它的已经不是默认模型，
// 调用方**无法分辨**（响应体里没有任何一格说「谁在应答」）。
//
// # 本件做什么（三件，缺一不可）
//
//	① 默认模型身份：env `ZERG_DEFAULT_MODEL`（点名且名册里真有）> 注册表 `default: true`；
//	② 自动上钉 + 续期：装载成功即 pin（带 TTL），由既有 2 秒巡检续期 —— **不新起常驻机制**，
//	   满足 Q5「无 TTL 的 pin 等同内存泄漏」与「常驻卵类别已废弃」（保护是 TTL，不是永久驻留）；
//	③ 禁止挤（fail-closed）：任何会驱逐它的装载在 `Start` 收口处被拒，并**逐字点名**
//	   「会驱逐钉住的默认模型 X，拒装 Y」；这是八条入口的全覆盖点。
//
// # 总闸
//
//	`ZERG_DEFAULT_PIN=off`（0/false/no/disabled 同义）⇒ 上钉、续期、拒装**三件事一起关**，
//	行为回到改动前（回滚只需设这一个环境变量，不必发版）。
package backend

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
)

const (
	// EnvDefaultPin 总闸：off/0/false/no/disabled ⇒ 关闭「自动上钉 + 冲突拒装」（缺省开）。
	EnvDefaultPin = "ZERG_DEFAULT_PIN"
	// EnvDefaultModel 默认模型身份覆盖（点名模型须在名册里；否则按「无默认」处理并告警）。
	// 用途：名册暂不方便改键的机器，以及测试/临时切换。
	EnvDefaultModel = "ZERG_DEFAULT_MODEL"
	// EnvDefaultPinTTL 上钉 TTL（秒）。缺省 DefaultPinTTL；非正整数 ⇒ 记日志并回落缺省。
	EnvDefaultPinTTL = "ZERG_DEFAULT_PIN_TTL_S"
	// DefaultPinTTL 上钉 TTL 缺省值：1800 秒（30 分钟）。
	//
	// 为什么不是「永久」：`registry.go` 明文写着「已废弃『常驻卵』类别 —— 默认空无例外」，
	// Q5 也写明「无 TTL 的 pin 等同内存泄漏」⇒ 保护只能是**有期限、可续期、可显式解除**的。
	// 续期由既有 2 秒巡检做（`PatrolResidency`），所以正常运行时它一直有效；
	// 巡检停摆（agentd 崩/重启）时它最多再护 30 分钟，不会成为永久占槽。
	DefaultPinTTL = 1800 * time.Second
)

// defaultPinEnabled 总闸判定（唯一读点）。
func defaultPinEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(EnvDefaultPin))) {
	case "off", "0", "false", "no", "disabled":
		return false
	}
	return true
}

// defaultPinTTL 读上钉 TTL（秒）；非法值不静默接受，记一条日志后回落缺省。
func (m *Manager) defaultPinTTL() time.Duration {
	raw := strings.TrimSpace(os.Getenv(EnvDefaultPinTTL))
	if raw == "" {
		return DefaultPinTTL
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		log.Printf("[backend] %s=%q 非法（须为正整数秒）——回落到 %s", EnvDefaultPinTTL, raw, DefaultPinTTL)
		return DefaultPinTTL
	}
	return time.Duration(n) * time.Second
}

// resolveDefaultModelName 定「本机默认模型」身份（构造时一次，此后只读）。
//
// 两条来源，env 优先（显式点名 > 名册声明）；两者都拿不到 ⇒ 空 = 不上钉（旧行为）。
// env 点名了一个名册里没有的模型 ⇒ **不猜**：告警 + 空（否则会钉住一个装不上的名字，
// 让所有装载都被拒 —— 那是把配置错误升级成服务停摆）。
func resolveDefaultModelName(reg *registry.Registry) string {
	env := strings.TrimSpace(os.Getenv(EnvDefaultModel))
	if env != "" {
		if reg != nil {
			if _, ok := reg.Get(env); !ok {
				log.Printf("[backend] ⚠ %s=%q 不在本机名册里 ⇒ 按「无默认模型」处理（不上钉、不拒装）", EnvDefaultModel, env)
				return ""
			}
		}
		log.Printf("[backend] 默认模型身份（来源 %s）: %s", EnvDefaultModel, env)
		return env
	}
	if reg == nil {
		return ""
	}
	def := reg.DefaultModel()
	if def != "" {
		log.Printf("[backend] 默认模型身份（来源 注册表 default: true）: %s", def)
	}
	return def
}

// DefaultModel 当前生效的默认模型名（观测面/测试用；空 = 未配置）。
func (m *Manager) DefaultModel() string {
	if m == nil {
		return ""
	}
	return m.defaultModel
}

// ensureDefaultPin 默认模型**就绪那一刻**上钉（幂等：pin 剩余已过半程则不重钉）。
//
// 调用点 = `Start` 的 defer（覆盖复用 / 合并等待 / 真加载三条 return）。
// 判据只认 ready / idle_armed（能服务的那两态）；loading / crashed 不上钉（它还不能服务，
// 上了钉反而会挡住正在进行的换模型）。
func (m *Manager) ensureDefaultPin(model string) {
	if m == nil || !defaultPinEnabled() {
		return
	}
	def := m.defaultModel
	if def == "" || model != def {
		return
	}
	ttl := m.defaultPinTTL()
	now := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()
	sp, ok := m.procs[model]
	if !ok || sp == nil {
		return
	}
	if sp.state != StateReady && sp.state != StateIdleArmed {
		return
	}
	if pinned, remain := pinState(sp, now); pinned && remain >= ttl.Seconds()/2 {
		return
	}
	sp.pinUntil = now.Add(ttl)
	log.Printf("[backend] 默认模型上钉: %s（TTL %s · 任何会驱逐它的装载都会被拒；总闸 %s=off 可关）",
		model, ttl, EnvDefaultPin)
}

// renewDefaultPin 续期（缺口 GAP-20261019-01 的第③件）。
//
// 挂在 `PatrolResidency` 的**既有 2 秒巡检**上（不新起 goroutine）：pin 剩余 < TTL/2 时续一次。
// 这样「长期占单槽」的风险被 TTL 限住，而正常运行又不会因为 TTL 到期让默认模型变回可驱逐。
func (m *Manager) renewDefaultPin(now time.Time) {
	if m == nil || !defaultPinEnabled() {
		return
	}
	def := m.defaultModel
	if def == "" {
		return
	}
	ttl := m.defaultPinTTL()

	m.mu.Lock()
	defer m.mu.Unlock()
	sp := m.procs[def]
	if sp == nil || (sp.state != StateReady && sp.state != StateIdleArmed) {
		return
	}
	if pinned, remain := pinState(sp, now); pinned && remain >= ttl.Seconds()/2 {
		return
	}
	sp.pinUntil = now.Add(ttl)
	log.Printf("[backend] 默认模型 pin 续期: %s（至 %s · 巡检驱动，无新常驻机制）",
		def, sp.pinUntil.Format(time.RFC3339))
}

// defaultPinConflict 判「这次装载会不会驱逐钉住的默认模型」——会 ⇒ 返回 fail-closed 的 409 体。
//
// 判据（都在锁内取，读的是**本端实际驻留**，不是路由表/声明）：
//   - 无默认模型 / 总闸 off / 装载的就是默认模型本身 / 默认模型根本没驻留 ⇒ 不冲突；
//   - 驻留上限已满（单槽 = 1 时必然满）⇒ 装新的就得先卸它 ⇒ 冲突；
//   - 服务类型不同（llama xor ds4）⇒ 本来就要强制清场 ⇒ 冲突。
//
// ★ 未知模型**不在这里改判**：让它继续走到 `doStart` 回 404（拒因必须可归因，409 不能
// 顶替「名册里没有这枚」）。
func (m *Manager) defaultPinConflict(model string) map[string]interface{} {
	if m == nil || !defaultPinEnabled() {
		return nil
	}
	def := m.defaultModel
	model = strings.TrimSpace(model)
	if def == "" || model == "" || model == def {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	sp := m.procs[def]
	if sp == nil || sp.state == StateCrashed || sp.state == StateDraining {
		return nil // 没驻留 / 已经不服务了 ⇒ 没有「会被驱逐」的对象
	}
	if m.registry == nil {
		return nil
	}
	target, ok := m.registry.Get(model)
	if !ok {
		return nil // 未知模型交给 doStart 的 404
	}
	limit := m.residentLimitLocked()
	wouldEvict := len(m.procs) >= limit
	if sp.entry != nil && serviceKind(sp.entry) != serviceKind(target) {
		wouldEvict = true
	}
	if !wouldEvict {
		return nil
	}
	_, remain := pinState(sp, time.Now())

	// 逐字点名：谁被驱逐、谁被拒、为什么、出路是什么。
	// 这句话是验收判据的一部分（调用方/运维据此分辨「装不上」与「被默认模型挡住了」）。
	msg := fmt.Sprintf("会驱逐钉住的默认模型 %s，拒装 %s（本机驻留上限=%d · pin 余 %.0fs）；要换：先 `zerg resource unpin`，或设 %s=off 关总闸后再装",
		def, model, limit, remain, EnvDefaultPin)
	log.Printf("[backend] ✗ %s", msg)
	return map[string]interface{}{
		"ok":           false,
		"status":       409,
		"code":         "pinned_resident_conflict",
		"error":        msg,
		"message":      msg,
		"pinned":       def,
		"pin_remain_s": int(remain),
		"requester":    model,
	}
}

// ServedInfo 「谁在应答」的如实读数（缺口 GAP-20261019-01 的可分辨面）。
//
// 只读 `m.procs`（本端真正托管的那一枚）—— 不由请求里的 model 推断、不由路由表推断。
type ServedInfo struct {
	Model      string
	Host       string
	Port       int
	Pid        int
	Pinned     bool
	PinRemainS float64
	Default    bool
}

// ServedInfo 取某模型此刻的应答身份；ok=false = 它没在服务（未驻留/未就绪）。
//
// 调用点 = `server.handleInfer` 写响应头处（`X-Zerg-Served-*`），必须在 `WriteHeader` 之前。
func (m *Manager) ServedInfo(model string) (ServedInfo, bool) {
	if m == nil {
		return ServedInfo{}, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	sp, ok := m.procs[model]
	if !ok || sp == nil || (sp.state != StateReady && sp.state != StateIdleArmed) {
		return ServedInfo{}, false
	}
	name := sp.model
	if name == "" {
		name = model
	}
	si := ServedInfo{
		Model:   name,
		Host:    m.machine,
		Port:    sp.port,
		Pid:     enginePIDOf(sp),
		Default: m.defaultModel != "" && name == m.defaultModel,
	}
	si.Pinned, si.PinRemainS = pinState(sp, time.Now())
	return si, true
}

// PinnedHeaderValue `X-Zerg-Served-Pinned` 的取值（闭集：default / pinned:<秒> / 空）。
//
// 三种态刻意分开：`default` = 这是默认模型（身份事实）；`pinned:N` = 它被钉住但**不是**默认
// （有人手工 pin）；空 = 没有任何保护。调用方据此判「我拿到的回答是不是被保护的那一枚」。
func (si ServedInfo) PinnedHeaderValue() string {
	if si.Default {
		return "default"
	}
	if si.Pinned {
		return fmt.Sprintf("pinned:%d", int(si.PinRemainS))
	}
	return ""
}
