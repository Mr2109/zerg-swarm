// service_borrow.go —— P3b-2：借用编排与归还看护。
//
// 设计依据：设计-子端服务切换与基线服务声明-20260914.md §7 S5（refuse|borrow 两档，Mr2109 定默认 borrow）、
// §9.3（借还序列）、§11 M1（崩溃兜底）/M5（max_hold）/M6（并发）/M7（最小足量集合）。
//
// 锁序（唯一合法顺序，务必遵守）：**先 m.mu，再 m.borrowMu**。
//   - m.borrowMu 保证同一时刻只有一次借/还编排（M6 的"不重复停服"）；
//   - 停服最长会占 grace_stop_s（默认 15s），所以借还**不新开 goroutine 抢 m.mu**，
//     而是在已持 m.mu 的装载路径内串行完成——期间其它模型操作阻塞是**正确的**
//     （借用中本来就不该有第二个模型动作）。
package backend

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/agent/internal/monitor"
)

// EnvServiceExclusive 借还档位：borrow（默认，Mr2109 2026-09-14 拍板）| refuse（只报告不动手）。
const EnvServiceExclusive = "ZERG_SERVICE_EXCLUSIVE"

// EnvLeaseTTLS 空闲 TTL（秒）；EnvMaxHoldS 借期绝对上限（秒）。
const (
	EnvLeaseTTLS = "ZERG_LEASE_TTL_S"
	EnvMaxHoldS  = "ZERG_MAX_HOLD_S"
)

// baselineReclaimMode 解析借还档位（默认 borrow）。
func baselineReclaimMode() string {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(EnvServiceExclusive)))
	if v == "refuse" {
		return "refuse"
	}
	return "borrow"
}

// returnLeases 的返回：每个已处理端口的简述（供日志/回执）。
type leaseOutcome struct {
	Port   int
	Action string // returned | skipped | failed
	Why    string
}

// startTimeMismatch 判定"发现时记录的 start_time"与"此刻读到的"是否不一致——
// 不一致说明 pid 已被复用（原进程已死、新进程占了同一 pid），**必须中止停止动作**（设计 R1）。
//
// 任一侧为 0（平台不提供 / 取不到）时返回 false：不因"取不到"而误伤，
// 但也如实记录（调用方可据此在日志里标注"守卫未生效"）。
func startTimeMismatch(archived, current uint64) bool {
	if archived == 0 || current == 0 {
		return false
	}
	return archived != current
}

// stopBaselineForBorrowLocked 停一个基线服务以腾坑：空闲检定 → **先存档** → 停 → 验停。
//
// 调用方必须已持 m.mu 与 m.borrowMu。**绝不停未声明的服务**（红线②）。
func (m *Manager) stopBaselineForBorrowLocked(svc BaselineService) (*ServiceLease, error) {
	// ① 空闲检定：有在飞请求就不借（绝不打断）
	idle, detail := probeServiceIdle(svc.Port, 3*time.Second)
	if !idle {
		return nil, errBorrowBlocked("目标服务正忙：" + detail)
	}

	// ② 先存档（write-then-act）：没有租约就没有停止动作
	lease := ServiceLease{
		Port:        svc.Port,
		Kind:        svc.Kind,
		Class:       svc.Class,
		ScreenName:  "",
		Unit:        svc.Unit,
		Identity:    svc.Identity,
		PID:         svc.PID,
		Argv:        svc.Argv,
		Cwd:         svc.Cwd,
		EnvFiltered: FilterEnvForLease(os.Environ()),
		StartTime:   svc.StartTime,
		ApproxGB:    svc.ApproxGB,
		State:       LeaseBorrowed,
		AcquiredAt:  time.Now(),
		LastActive:  time.Now(),
		TTLS:        int(envFloat(EnvLeaseTTLS, float64(DefaultLeaseTTLS))),
		MaxHoldS:    int(envFloat(EnvMaxHoldS, float64(DefaultMaxHoldS))),
		Note:        "borrow：按 baseline 声明腾坑（停前已存档，到期自动归还）",
	}
	if len(svc.Argv) == 0 {
		return nil, errBorrowBlocked("未取到目标服务命令行，拒绝停止（无法归还）")
	}
	// 身份守卫（安全关键）：端口上必须**确实是推理服务**才允许停。
	// 否则"某个碰巧占着 9000 的无关服务"会被当成基线服务停掉——这是不可接受的越界。
	if !isInferenceArgv(svc.Argv) {
		return nil, errBorrowBlocked(fmt.Sprintf("端口 %d 上的进程不是推理服务（argv 不符），拒绝停止", svc.Port))
	}
	// PID 复用守卫（设计 R1）：发现时记的 start_time 与此刻读到的不一致 ⇒ 目标已换人，
	// 中止停止（旧进程已死、pid 被新进程复用，照着旧记录停就会误杀无辜）。
	if cur := currentStartTime(svc.PID); startTimeMismatch(svc.StartTime, cur) {
		return nil, errBorrowBlocked(fmt.Sprintf(
			"pid %d 已被复用（start_time %d → %d），拒绝停止（恐怕停错东西）", svc.PID, svc.StartTime, cur))
	}
	if err := SaveLease(lease); err != nil {
		return nil, errBorrowBlocked("存档失败，拒绝停止：" + err.Error())
	}

	// ③ 按托管方式停
	var stopErr error
	switch svc.Class {
	case "systemd":
		stopErr = stopSystemdUnit(svc.Unit)
	case "screen":
		// screen 会话名从祖先链里取（P2 的 BaselineService 不直接带，用 argv 兜底判断）
		stopErr = m.stopByClassOrTree(svc, &lease)
	default:
		stopErr = m.stopByClassOrTree(svc, &lease)
	}
	if stopErr != nil {
		lease.State = LeaseRestoreFailed
		lease.Note = "停止失败：" + stopErr.Error()
		_ = SaveLease(lease)
		return nil, stopErr
	}

	// ④ 验停：端口不再应答
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if !listeningOn(svc.Port) {
			lease.LastActive = time.Now()
			_ = SaveLease(lease)
			log.Printf("[baseline] 已借用 :%d（%s/%s，%s）——租约 ttl=%ds max_hold=%ds",
				svc.Port, svc.Kind, svc.Class, svc.Identity, lease.TTLS, lease.MaxHoldS)
			return &lease, nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	lease.State = LeaseRestoreFailed
	lease.Note = "停止后端口仍在监听（可能有其它实例）"
	_ = SaveLease(lease)
	return nil, errBorrowBlocked("停止后端口仍在监听，暂不借用（租约已留存）")
}

// stopByClassOrTree 非 systemd 类：按 pid 停整棵进程树。
func (m *Manager) stopByClassOrTree(svc BaselineService, lease *ServiceLease) error {
	if svc.PID <= 0 {
		return errBorrowBlocked("未定位到进程，拒绝停止（无法保证停对东西）")
	}
	killed, err := stopProcessTree(svc.PID, time.Duration(DefaultGraceStopS)*time.Second)
	lease.Pipeline = killed
	_ = SaveLease(*lease)
	return err
}

// borrowNeeded 纯函数：给定"需要多少"与"当前可用多少"，判断要不要借、缺多少。
func borrowNeeded(needGB, availGB float64) (bool, float64) {
	if needGB-availGB <= 0 {
		return false, 0
	}
	return true, needGB - availGB
}

// tryBorrowForMemoryLocked 内存不足时，按"最小足量集合"借用基线服务的坑（M7）。
//
// 返回已借用的端口列表。fail-closed：任何一个环节不满足就整体放弃（宁可拒装，也不半借）。
func (m *Manager) tryBorrowForMemoryLocked(needGB float64) ([]int, error) {
	// 先算缺口：**没有缺口就什么都不做、也不报错**（调用方只在内存不足时才调本函数）。
	systemAvail := monitor.DefaultSampler.MemAvailableGb()
	// fail-safe：采样不可用（非 Linux 平台可能返回 0）时**绝不借**——
	// 绝不基于"未知"去停别人手工起的服务（红线②的精神）。
	if systemAvail <= 0 {
		return nil, errBorrowBlocked("系统可用内存采样不可用（返回 ≤0）⇒ 不借用")
	}
	avail := m.effectiveAvailableGbLocked(systemAvail)
	needed, deficit := borrowNeeded(needGB, avail)
	if !needed {
		return nil, nil
	}
	if baselineReclaimMode() == "refuse" {
		return nil, errBorrowBlocked("档位为 refuse（只报告不动手，见设计 §7 S5）")
	}
	m.borrowMu.Lock()
	defer m.borrowMu.Unlock()

	svcs := m.baselineServicesLocked()
	if len(svcs) == 0 {
		return nil, errBorrowBlocked("未声明任何基线服务（ZERG_BASELINE_PORTS 为空）")
	}
	// 候选：在监听且能在免 root 下停的（裸进程 / screen；systemd 需授权，先试一次再看错误）
	var cands []BaselineService
	for _, s := range svcs {
		if s.Listening {
			cands = append(cands, s)
		}
	}
	// 最小足量：按占用从大到小取，凑够 deficit 即止（M7）
	sort.Slice(cands, func(i, j int) bool { return cands[i].ApproxGB > cands[j].ApproxGB })

	var borrowed []int
	freed := 0.0
	for _, s := range cands {
		if freed >= deficit {
			break
		}
		l, err := m.stopBaselineForBorrowLocked(s)
		if err != nil {
			// 已经借出去的必须还回去（不允许半借状态）
			for _, p := range borrowed {
				if ll, ok := LoadLease(p); ok {
					_ = m.ReturnLease(ll)
				}
			}
			return nil, err
		}
		borrowed = append(borrowed, s.Port)
		freed += l.ApproxGB
		log.Printf("[baseline] 借出 :%d 释放 ≈%.1fGB（目标缺口 %.1fGB）", s.Port, l.ApproxGB, deficit)
	}
	if len(borrowed) == 0 {
		return nil, errBorrowBlocked("没有可借用的基线服务（都在忙或未声明）")
	}
	return borrowed, nil
}

// ReturnLease 归还一个租约：原样重放 + 验身份；成功则删租约并写回执。
//
// 「不与人争」：归还前若该端口已被**同一身份**的服务占着（用户自己起回来了）⇒ 视为已归还。
func (m *Manager) ReturnLease(l *ServiceLease) error {
	if l == nil {
		return nil
	}
	m.borrowMu.Lock()
	defer m.borrowMu.Unlock()

	if listeningOn(l.Port) {
		got := probeIdentity(l.Port, 3*time.Second)
		if l.Identity != "" && got == l.Identity {
			log.Printf("[baseline] :%d 已是同一服务（用户自己起回来了）⇒ 视为已归还", l.Port)
			return DeleteLease(l.Port)
		}
		// 端口被别的服务占着 ⇒ 不能盲目重放（会撞端口）
		l.State = LeaseRestoreFailed
		l.Note = "归还前发现端口已被其它服务占用：期望 " + l.Identity + "，实得 " + got
		_ = SaveLease(*l)
		return errBorrowBlocked(l.Note)
	}

	if err := restoreBaselineService(*l, time.Duration(DefaultRestoreWaitS)*time.Second); err != nil {
		l.State = LeaseRestoreFailed
		l.Note = "归还失败：" + err.Error()
		_ = SaveLease(*l)
		log.Printf("[baseline] ✗ 归还 :%d 失败：%v（租约保留，下次启动/watchdog 会重试）", l.Port, err)
		return err
	}
	log.Printf("[baseline] ✓ 已归还 :%d（%s）", l.Port, l.Identity)
	return DeleteLease(l.Port)
}

// recomputeAfterBorrow 借用成功后重算可用内存（n16）。
//
// 为什么不能只用采样值：刚停掉基线服务时，内核回收 GTT/页需要时间，采样会**滞后**
// （2026-09-14 真机：借用释放了 32.7GB，采样仍报 61.0GB 旧值）⇒ 白借一次、回 507、
// 租约空悬到 TTL 才被回收。
// 为什么也不能直接相加：采样若已跟上，再加上"腾出量"就是重复计数。
// ⇒ 取两者较大者：滞后时用推算托底，采样跟上后自然切回采样值。两个口径都不会被高估。
//
// 纯函数，便于单测（不碰采样器、不碰进程）。
func recomputeAfterBorrow(sampled, before, freedGb float64) float64 {
	if projected := before + freedGb; projected > sampled {
		return projected
	}
	return sampled
}

// borrowedOccupiedGbLocked 求"本次借出的那几个端口"原本的实测占用合计（GB）。
//
// ⚠️ 必须从**租约存档**取，不能从 baselineServicesLocked() 取 —— 这是 2026-09-14 端到端复验
// 抓到的真 bug：服务一旦被停，身份探测（/v1/models）就失败 ⇒ 它不再出现在基线列表里
// ⇒ 求和恒为 0（真机日志原文：`借用后重算：采样 61.4 GB、本次腾出 0.0 GB ⇒ 取 61.4 GB`
// ⇒ 仍判内存不足、白借一次）。租约里的 ApproxGB 是**停机前**记下的，才是真值。
func (m *Manager) borrowedOccupiedGbLocked(ports []int) float64 {
	var sum float64
	for _, p := range ports {
		if l, ok := LoadLease(p); ok {
			sum += l.ApproxGB
		}
	}
	return sum
}

// ReturnAllBorrowedLeases 启动恢复：把**所有**租约当作"该归还"处理（§9.3 第 8 步的真实语义）。
//
// 为什么不能复用 ReturnExpiredLeases（2026-09-14 N8 真机演练的教训）：
//
//	`ReturnExpiredLeases` 是**过期扫描** —— 只有空闲超过 ttl 或达到 max_hold 才动。
//	而崩溃恢复的场景是"进程重启 ⇒ 借的人已经不在了"，此时**不该等 TTL**。
//	真机实测：kill -9 时租约才诞生 ~110s，而 TTL=300s ⇒ 被跳过 ⇒ :9000 一直停着，
//	180s 观察窗内一次都没归还（最后靠手工用存档 argv 才拉回）。
//
// 覆盖两种必须立即处理的：① borrowed（无论新旧）；② restore_failed（上次归还失败，立即重试）。
func (m *Manager) ReturnAllBorrowedLeases() []leaseOutcome {
	var out []leaseOutcome
	for _, l := range ListLeases() {
		if l.Port <= 0 {
			continue
		}
		if l.State != LeaseBorrowed && l.State != LeaseRestoreFailed {
			continue
		}
		why := "启动恢复：进程重启 ⇒ 借用方已不在，立即归还"
		if l.State == LeaseRestoreFailed {
			why = "启动恢复：上次归还失败，立即重试"
		}
		lease := l
		if err := m.ReturnLease(&lease); err != nil {
			out = append(out, leaseOutcome{Port: l.Port, Action: "failed", Why: why + "；" + err.Error()})
			continue
		}
		out = append(out, leaseOutcome{Port: l.Port, Action: "returned", Why: why})
	}
	return out
}

// ReturnExpiredLeases 扫描全部租约，归还到期的（wall-clock 判定 ⇒ 崩溃后任何进程都能执行）。
func (m *Manager) ReturnExpiredLeases(now time.Time) []leaseOutcome {
	var out []leaseOutcome
	for _, l := range ListLeases() {
		should, why := LeaseActionable(l, now)
		if !should {
			continue
		}
		lease := l
		if err := m.ReturnLease(&lease); err != nil {
			out = append(out, leaseOutcome{Port: l.Port, Action: "failed", Why: why + "；" + err.Error()})
			continue
		}
		out = append(out, leaseOutcome{Port: l.Port, Action: "returned", Why: why})
	}
	return out
}

// StartLeaseWatchdog 启动租约看护（M1 的兜底通道）：每 interval 扫一次到期租约并归还。
//
// 与"子端崩溃后由下次启动归还"互补：只要子端还活着，租约一定不会过期无人管。
func (m *Manager) StartLeaseWatchdog(interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	m.mu.Lock()
	if m.leaseWatchStop != nil { // 已启动
		m.mu.Unlock()
		return
	}
	stop := make(chan struct{})
	m.leaseWatchStop = stop
	m.mu.Unlock()

	// 启动即**立即归还所有 borrowed 租约**（§9.3 第 8 步）——崩溃重启意味着借用方已不在，**不该等 TTL**。
	// 2026-09-14 N8 真机教训：此处原用 ReturnExpiredLeases（只还"已过期"的）
	// ⇒ kill -9 时租约才 ~110s、TTL 300s ⇒ 被跳过 ⇒ 服务一直停着（实测 6 分钟没回来）。
	// 两个扫描**串行**跑：先立即归还，再补一次过期扫描（幂等；串行可避免两个 goroutine 抢同一份租约）。
	go func() {
		for _, o := range m.ReturnAllBorrowedLeases() {
			log.Printf("[baseline] 启动恢复：:%d %s（%s）", o.Port, o.Action, o.Why)
		}
		for _, o := range m.ReturnExpiredLeases(time.Now()) {
			log.Printf("[baseline] 启动扫描：:%d %s（%s）", o.Port, o.Action, o.Why)
		}
	}()
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				for _, o := range m.ReturnExpiredLeases(time.Now()) {
					log.Printf("[baseline] watchdog：:%d %s（%s）", o.Port, o.Action, o.Why)
				}
			}
		}
	}()
}

// StopLeaseWatchdog 停止看护（测试与优雅退出用）。
func (m *Manager) StopLeaseWatchdog() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.leaseWatchStop != nil {
		close(m.leaseWatchStop)
		m.leaseWatchStop = nil
	}
}

// ── 小工具 ─────────────────────────────────────────────────────────────────

// borrowBlockedError 借用被拒（不是故障，是可解释的"先别装"）。
type borrowBlockedError struct{ why string }

func (e borrowBlockedError) Error() string { return e.why }

func errBorrowBlocked(why string) error { return borrowBlockedError{why: why} }

// IsBorrowBlocked 判定错误是否为"可解释的借用受阻"（调用方据此回 507 并打印原因）。
func IsBorrowBlocked(err error) bool {
	_, ok := err.(borrowBlockedError)
	return ok
}

// ReclaimMode 对外只读：当前借还档位（borrow | refuse）。
func ReclaimMode() string { return baselineReclaimMode() }

// DeclaredBaselinePorts 对外只读：baseline 声明的端口清单（未声明返回空）。
func DeclaredBaselinePorts() []int { return baselinePorts() }
