// hatch_path.go —— 批 2：把孵化器接进 backend 的孵化 / 收卵流程（开关 ZERG_HATCH，**默认关**）。
//
// 设计真源：《设计-子端隔离化-20260914》（仓库内该文档标题含旧词，按既有先例以「隔离化」指代）
//
//	§6.1 孵化形态（bwrap 管封闭空间 + systemd-run --user 管归属）
//	§6.5 生命周期（收卵 = 停单元；卵不活过子端重启）
//	§6.9 静默失效不得当凭据（孵化后必须运行时核，不以单元状态为凭）
//	§7.4 就绪两层（沿用既有 waitForReady + 功能预检，不新造就绪判据）
//	§8.4 准入闸门（双闸门 GTT 账 + 内存账，fail-closed）
//
// 开关语义（写死）：
//
//	未设 / 空 / "0"         ⇒ **关（缺省）**：doStart / Stop / ReapIdle 走原有句柄路径，
//	                          一行代码路径都不变（X3 未验证前不得改变现有行为）；
//	"1" / "true" / "yes" / "on" ⇒ 开：doStart 先判实测档案与两账（不过 ⇒ 507 拒孵，不起任何单元）
//	                          → hatch.Hatcher.Hatch 起单元 → 既有就绪判据 → 核封闭性
//	                          （三态：实读不符 ⇒ 收卵 + 拒孵；读不到 ⇒ 告警 + 观测面标记，不拒服务）；
//	                          收卵改走 Hatcher.Collect（幂等）。
//
// 开关关时如何保证「一行不变」：只有开关开的路径才会写 subproc.Unit；收卵一律以「Unit 非空」
// 为判据（开关关 ⇒ 恒空 ⇒ 走既有句柄路径）。开关**中途被关掉**也不会漏收已孵的卵——收卵看的
// 是 sp.Unit，不是当前开关值。
package backend

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/Mr2109/zerg-swarm/agent/internal/hatch"
	"github.com/Mr2109/zerg-swarm/agent/internal/monitor"
	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
)

// EnvHatch 孵化路径的开关（**默认关**；见文件头语义）。
const EnvHatch = "ZERG_HATCH"

// EnvGttBudgetGb 全局 GTT 预算（GB）——§8.4：gtt_free = gtt_budget − gtt_used（− reserve_gb）。
//
// 数值**待 P3 实测标定**（设计 Q10：预算项需一组默认值，必须由标定流程实测得出）⇒
// 未配置即视为「账读不到」：闸门 fail-closed 拒孵，**绝不**拿估值 / 卵声明值当预算（标定铁律 §8.4）。
const EnvGttBudgetGb = "ZERG_MACHINE_GTT_BUDGET_GB"

// hatchEnabledFromEnv 开关判据（纯函式，便于单测注入 getenv）：只有显式的「真值」才开。
// 读不到 / 空 / 其它值一律视为**关**——默认关是硬要求（改了它就是改了现有行为）。
func hatchEnabledFromEnv(getenv func(string) string) bool {
	if getenv == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(getenv(EnvHatch))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// hatchEnabled 读环境得到本进程的开关值。
func hatchEnabled() bool { return hatchEnabledFromEnv(os.Getenv) }

// hatcher 孵化器的小接口（只列本接线用到的五件事）。
//
// 存在理由只有一个：让「开关开」的行为可被单测——注入假孵化器即可验证「单元名记进 subproc /
// 收卵走 Collect / 无档案不起任何单元 / 拿不到 pid 记为未核验」，而不必在开发机上拉起真实
// systemd（真 systemd 依赖只在 X3 上存在）。
type hatcher interface {
	Hatch(ctx context.Context, spec hatch.Spec) (string, error)
	Collect(ctx context.Context, unit string) error
	Active(ctx context.Context, unit string) (bool, error)
	MainPID(ctx context.Context, unit string) (int, error)
	VerifyEnclosure(pid int) (hatch.EnclosureReport, error)
	// VerifyEnclosureDeclared 带**卵声明的权重文件清单**的核验（2026-09-15 接线）：
	// 判据从「结构形态」收紧到「正好是这几个声明文件各自一条只读挂载」——
	// 否则「挂了别的文件」在核验里看不见（§6.9：看得见的才算凭据）。
	VerifyEnclosureDeclared(pid int, declaredWeightFiles []string) (hatch.EnclosureReport, error)
}

// 编译期断言：真孵化器（Linux 真实现 / 非 Linux 明确拒绝桩）必须满足该接口。
var _ hatcher = hatch.Hatcher{}

// hatcherImpl 取本次要用的孵化器（测试注入的优先；生产恒为 hatch.Hatcher{}）。
func (m *Manager) hatcherImpl() hatcher {
	if m != nil && m.hatcher != nil {
		return m.hatcher
	}
	return hatch.Hatcher{}
}

// ── 双闸门与实测档案（§8.4 / §6.8.4「先判格式，再算账」）──────────────────────

var (
	hatchVitalsOnce sync.Once
	hatchVitalsRec  *monitor.VitalsRecorder
)

// hatchVitalsSnapshot 后端自用的虫须**只读快照**（不启 goroutine、不落盘、不入环，§5.4 口径）。
//
// 为什么是后端自起一台：server.NewAgent 那台 *monitor.VitalsRecorder 只给观测面用，backend
// 拿不到实例（Manager 无该字段）⇒ 若要接 OnHatch/OnCollect 钩子，需要 server 侧把实例交给
// backend（见文件末 TODO）。闸门只是「读一次数」，自起一台只读实例即可（读数不依赖实例共享）。
func hatchVitalsSnapshot() monitor.Vitals {
	hatchVitalsOnce.Do(func() { hatchVitalsRec = monitor.NewVitalsRecorder() })
	return hatchVitalsRec.Snapshot()
}

// gttBudgetGbFromEnv 读全局 GTT 预算（GB）：未配置 / 非法 ⇒ ok=false（**不拿估值凑**）。
func gttBudgetGbFromEnv(getenv func(string) string) (float64, bool) {
	if getenv == nil {
		return 0, false
	}
	raw := strings.TrimSpace(getenv(EnvGttBudgetGb))
	if raw == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || v <= 0 {
		log.Printf("[backend] %s=%q 非法（须为正数 GB）——按「未标定」处理（闸门 fail-closed）", EnvGttBudgetGb, raw)
		return 0, false
	}
	return v, true
}

// hatchGateRead 读双闸门的两侧可用（GTT 可用 GB / 内存可用 GB）。读不到 ⇒ 报错（调用方拒孵）。
//
// 抽成变量是为了可测：单测注入确定的一对数，就不必依赖开发机的 sysfs / 真实内存。
var hatchGateRead = func() (float64, float64, error) {
	budget, ok := gttBudgetGbFromEnv(os.Getenv)
	if !ok {
		return 0, 0, fmt.Errorf("GTT 预算未标定（%s 未配置）——标定铁律：闸门预算必须实测得出（§8.4 / Q10）",
			EnvGttBudgetGb)
	}
	v := hatchVitalsSnapshot()
	if !v.Gtt.Ok {
		return 0, 0, fmt.Errorf("本机读不到全局 GTT（非 Linux 或 sysfs 不可读）——账读不到就不放行")
	}
	if !v.Mem.Ok {
		return 0, 0, fmt.Errorf("本机读不到内存账（MemAvailable）——账读不到就不放行")
	}
	return budget - v.Gtt.UsedGb(), v.Mem.AvailGb, nil
}

// hatchGateLocked 孵化前的双闸门 + 实测档案（调用方需持锁；与既有内存预检同一处判）。
//
// 返回 (实测档案, nil) = 放行；(零值, 507 响应) = 拒孵。三条 fail-closed：
//   - 实测档案读不出 / 校验不过 ⇒ 无实测档案，拒孵（§8.4 标定铁律）；
//   - 两账读不到（预算未标定 / 本机读不到 GTT） ⇒ 拒孵（算不出就不装）；
//   - 两账不够 ⇒ 507 + 列明「谁挡着 / 差多少」。
//
// **拒孵一律不起任何单元**（本函数只读账、只读档案，没有任何动作）。
func (m *Manager) hatchGateLocked(modelName string, entry *registry.ModelEntry) (monitor.EggProfile, map[string]interface{}) {
	eggID := strings.TrimSpace(entry.EggName())
	profilePath := monitor.EggProfilePath(eggID)
	var (
		prof       monitor.EggProfile
		profileErr error
	)
	if profilePath == "" {
		profileErr = fmt.Errorf("卵名拿不到，实测档案路径算不出来")
	} else {
		prof, profileErr = monitor.LoadEggProfile(profilePath)
	}
	gttAvail, memAvail, acctErr := hatchGateRead()

	profileOk := profileErr == nil
	if !profileOk || acctErr != nil {
		// 纯函式裁决：档案无效 ⇒ 一律不放行（理由以它为准，口径与 P4 闸门同源）
		dec := monitor.CanHatchWithProfile(false, 0, 0, monitor.EggProfile{})
		detail := dec.Reason
		code := "no measured profile"
		switch {
		case !profileOk:
			detail = fmt.Sprintf("%s（档案=%s：%v）", dec.Reason, profilePath, profileErr)
		case acctErr != nil:
			code = "hatch gate unreadable"
			detail = fmt.Sprintf("闸门两账读不到，拒孵（fail-closed：算不出就不装）：%v", acctErr)
		}
		log.Printf("[backend] ✗ 拒孵 %s（孵化闸门）：%s", modelName, detail)
		return monitor.EggProfile{}, errResponse(507, code, detail)
	}

	if res := monitor.CanHatchWithProfile(true, gttAvail, memAvail, prof); !res.Ok {
		detail := fmt.Sprintf("%s（GTT 可用 %.1f GB / 内存可用 %.1f GB；实测档案 peak_gtt=%.1f GB peak_mem=%.1f GB）",
			res.Reason, gttAvail, memAvail, prof.PeakGttGb, prof.PeakMemGb)
		log.Printf("[backend] ✗ 拒孵 %s（双闸门未过）：%s", modelName, detail)
		return monitor.EggProfile{}, errResponse(507, "insufficient memory", detail)
	}
	log.Printf("[backend] 双闸门通过: %s（GTT 可用 %.1f GB / 内存可用 %.1f GB；档案 peak_gtt=%.1f GB peak_mem=%.1f GB）",
		modelName, gttAvail, memAvail, prof.PeakGttGb, prof.PeakMemGb)
	return prof, nil
}

// ── 孵化路径（开关开时 doStart 走这里）──────────────────────────────────────

// hatchStartLocked 孵化一枚卵：映射声明 → 起单元 → 既有就绪判据 → 核封闭性。
// 与裸 exec 路径的分工：本函数只负责「起 / 等 / 核」，**收卵**在 stopSubproc / stopVictimProcess
// （按 sp.Unit 分派）。调用方需持锁（端口与子进程状态都在同一临界区里定，与裸 exec 路径一致）。
func (m *Manager) hatchStartLocked(modelName string, entry *registry.ModelEntry, sp *subproc, port int, profile monitor.EggProfile) (map[string]interface{}, error) {
	spec, err := hatchSpecFor(entry, EngineImplOf(entry), port, profile)
	if err != nil {
		log.Printf("[backend] ✗ 拒孵 %s：孵化声明映射不出来：%v", modelName, err)
		sp.state = StateCrashed
		delete(m.procs, modelName)
		return errResponse(502, "hatch spec unavailable", err.Error()), nil
	}
	// 声明校验（schema_version / 引擎路径 / 权重 / 档案）——认不得就报错，**不回退**裸 exec（§6.8.4）
	if err := spec.Validate(); err != nil {
		log.Printf("[backend] ✗ 拒孵 %s：孵化声明校验不过：%v", modelName, err)
		sp.state = StateCrashed
		delete(m.procs, modelName)
		return errResponse(502, "hatch spec rejected", err.Error()), nil
	}

	h := m.hatcherImpl()
	unit, err := h.Hatch(context.Background(), spec)
	if err != nil {
		log.Printf("[backend] 孵化失败: model=%s unit=%s: %v", modelName, unit, err)
		sp.state = StateCrashed
		return errResponse(500, "failed to start backend", err.Error()), nil
	}
	sp.Unit = unit
	log.Printf("[backend] 孵化单元已建: model=%s unit=%s port=%d 权重=%s 引擎=%s 工作目录=%s（空间内）可写绑定=%v",
		modelName, unit, port, spec.WeightPath, spec.EnginePathInSpace, spec.WorkDir, spec.ExtraRWBinds)

	// 就绪：沿用既有判据（§7.4 两层——端口/健康检查 + 一次真实生成的功能预检），不新造一套
	if err := m.waitForReady(sp); err != nil {
		log.Printf("[backend] 孵化后健康检查失败: %v", err)
		m.stopSubproc(sp)
		sp.state = StateCrashed
		return errResponse(500, "health check failed", err.Error()), nil
	}
	if probeErr := probeInference(port, modelName, probeTimeout()); probeErr != nil {
		log.Printf("[backend] 功能预检未通过（孵化）: model=%s port=%d unit=%s: %v", modelName, port, unit, probeErr)
		m.stopSubproc(sp)
		sp.state = StateCrashed
		delete(m.procs, modelName)
		return errResponse(502, "model unusable after load (functional probe failed)", probeErr.Error()), nil
	}

	// §6.9：孵化后**运行时核验**封闭性（读 /proc/<pid>/mountinfo，不以单元状态为凭）。
	// **三态分级**（Mr2109 2026-09-15 拍）：
	//   - 实读且不符 ⇒ 立刻**收卵 + 拒孵**（下面这一段），绝不继续对外服务；
	//   - 读不到     ⇒ 告警 + 在观测面标出（enclosure_verified=false + note），**不拒服务**；
	//   - 通过       ⇒ enclosure_verified=true。
	est, note := m.verifyEnclosure(unit, spec.WeightFiles)
	sp.enclosureVerified = est == enclosureVerified
	sp.enclosureNote = note
	if est == enclosureMismatch {
		log.Printf("[backend] ✗ 收卵 + 拒孵 %s（封闭性核验不符）: unit=%s %s", modelName, unit, note)
		m.stopSubproc(sp) // 收卵（sp.Unit 非空 ⇒ 走 Hatcher.Collect，幂等）
		sp.state = StateCrashed
		delete(m.procs, modelName)
		return errResponse(502, "enclosure verification failed",
			fmt.Sprintf("封闭空间核验不符，已收卵、不对外服务：%s", note)), nil
	}

	sp.failCnt = 0
	sp.state = StateReady
	log.Printf("[backend] 后端就绪（孵化路径）: unit=%s port=%d model=%s 封闭性=%s", unit, port, modelName, sp.enclosureNote)
	return okResponse(modelName, entry.Backend, port), nil
}

// enclosureState 封闭性核验的三种结局（**三态，不许压成一个布尔**）。
//
// 为什么必须分开（§6.9「静默失效不得当凭据」）：「没读到证据」与「读到反证」是两回事 ——
// 前者只能说「不知道」，后者是「确知没生效」。把两者混成一个 false 会得出「都在告警，大概没事」，
// 而把「没读到」当「通过」则是本项目明令禁止的静默失效。
type enclosureState int

const (
	// enclosureVerified 实读 mountinfo 且判定通过。
	enclosureVerified enclosureState = iota
	// enclosureUnreadable 拿不到 pid / 读不到 mountinfo / 解析不了 ⇒ 告警 + 观测面标出，
	// **不拒服务**（卵照常对外服务，但它那条「已核验」的宣称不成立）。
	enclosureUnreadable
	// enclosureMismatch 实读且判定不符 ⇒ **收卵 + 拒孵**（对外不服务）。
	enclosureMismatch
)

// verifyEnclosure 孵化后的封闭性核验（§6.9 硬要求）。
//
// 三种结局都如实记账，绝不混为一谈（§6.9 第 3 条：配置「被接受」与「生效」是两件事）：
//  1. **实读且不符**（拿到 mountinfo，判定不满足：/models 不是只读、宿主路径在空间内可见、
//     /work 或 /kvdisk 不是 rw…）⇒ enclosureMismatch：由调用方**收卵 + 拒孵**；
//  2. **读不到**（拿不到 pid / pid 已死 / 读不到或解析不了 mountinfo）⇒ enclosureUnreadable：
//     **不拒服务**，但必须**在观测面标出来**（enclosure_verified=false + enclosure_note）；
//  3. **通过** ⇒ enclosureVerified（观测面 enclosure_verified=true）。
//
// 为什么「读不到」也要上观测面、而不只是打日志：本项目硬要求「静默失效不得当凭据」（§6.9）——
// 声称隔离生效却读不到证据，必须是一个**看得见**的状态；日志会被冲掉、也没人翻。
//
// 返回 (状态, 一句话留痕)：留痕原样进观测面的 enclosure_note（错误信息里也会带上同一句话）。
func (m *Manager) verifyEnclosure(unit string, declaredWeightFiles []string) (enclosureState, string) {
	h := m.hatcherImpl()
	pid, err := h.MainPID(context.Background(), unit)
	if err != nil || pid <= 0 {
		note := fmt.Sprintf("未核验：拿不到引擎 pid（unit=%s）：%v", unit, err)
		log.Printf("[backend] ⚠ 封闭性核验未执行: %s —— §6.9：未核验不等于通过（本卵照常服务，但「已核验」不成立）", note)
		return enclosureUnreadable, note
	}
	rep, err := h.VerifyEnclosureDeclared(pid, declaredWeightFiles)
	if err != nil {
		note := fmt.Sprintf("未核验：读不到 pid=%d 的 mountinfo：%v", pid, err)
		log.Printf("[backend] ⚠ 封闭性核验未执行: unit=%s %s —— §6.9：未核验不等于通过（本卵照常服务，但「已核验」不成立）",
			unit, note)
		return enclosureUnreadable, note
	}
	if rep.Enclosed() {
		note := rep.String()
		log.Printf("[backend] 封闭性核验通过: unit=%s pid=%d %s", unit, pid, note)
		return enclosureVerified, note
	}
	note := fmt.Sprintf("核验不符：%s（%s）", strings.Join(rep.Failures(), "；"), rep)
	log.Printf("[backend] ✗ 封闭性核验**未通过**（§6.9 静默失效形态）: unit=%s pid=%d %s —— 收卵 + 拒孵", unit, pid, note)
	return enclosureMismatch, note
}

// ── 收卵（开关开时走孵化器 Collect；开关关时 sp.Unit 恒空 ⇒ 走既有句柄路径）──────

// collectUnit 收卵：停单元（幂等；systemctl --user stop）。
// 幂等语义在 hatch 包（二次停视为成功）；这里只如实记日志——收不掉必须留痕。
//
// 收完**再问一次**单元是否真在跑（单元状态不是凭据、退出码也不是，§6.9 同一精神：
// 「命令成功返回」只说明命令跑完了）——它还在跑就必须留痕，不许把「停命令成功」当「收干净了」。
func (m *Manager) collectUnit(unit string) {
	if strings.TrimSpace(unit) == "" {
		return
	}
	h := m.hatcherImpl()
	ctx := context.Background()
	if err := h.Collect(ctx, unit); err != nil {
		log.Printf("[backend] 收卵失败: unit=%s: %v", unit, err)
		return
	}
	if active, err := h.Active(ctx, unit); err == nil && active {
		log.Printf("[backend] ⚠ 收卵后单元仍在跑: unit=%s —— §6.9：Collect 报成功不等于收干净了", unit)
		return
	}
	log.Printf("[backend] 收卵完成: unit=%s", unit)
}

// TODO（批 2 未接，如实记）：
//   - 虫须钩子（OnHatch / OnCollect / MaybeCollect）未接：backend 拿不到 server.NewAgent 持有的
//     *monitor.VitalsRecorder（Manager 无该字段）⇒ 需要 server 侧把实例交给 backend（如 SetVitals）
//     才能接「孵化前后各取一次体征」；本批不擅自加一个没人调用的 setter。
//   - 收卵后的 GTT 归零校验（§8.5）同理：需要虫须实例 + 逐进程归因，归 P4 观测面接线。
