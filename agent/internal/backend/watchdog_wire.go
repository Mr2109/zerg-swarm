package backend

// watchdog_wire.go —— 批 B（T4/T5）：把活性看门狗接进转发路径，并定"判死"的对外语义。
//
// 设计真源：Zerg-内部文档/01-设计/设计-活性看门狗.md §2/§6/§7
//   · 证据源 = **本单元** cgroup 的累计 CPU 工时（按单元取，整机指标会被隔壁卵作伪证）；
//   · 判死 ⇒ 置 crashed（不再派新请求）+ **不收卵**（是否收卵交给既有五档裁决）；
//   · pin 未到期的卵 ⇒ **只告警不判死**（Q7）；
//   · 判死/忙一律 `503 + Retry-After`，并把看门狗判词与理由透给上游（主控 failover + 人工复盘）。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/agent/internal/monitor"
)

// eggCPUUsecAt 读 cgroup 路径（形如 /user.slice/.../<unit>.service）对应的累计 CPU 工时。
// cgroup v2 的统一挂载点 = /sys/fs/cgroup。
func eggCPUUsecAt(cgroupPath string) (uint64, error) {
	return monitor.EggCPUUsecFromCgroup("/sys/fs/cgroup" + cgroupPath)
}

// BackendBusyError 后端忙/被判死——**可重试语义**（不是"机器坏了"）。
//
// 服务端见它回 503 + Retry-After，并把 Obs（判词/理由/窗口/工时增量）写进响应体，
// 让主控能据此 failover、人能直接看懂为什么。
type BackendBusyError struct {
	Verdict WatchdogVerdict
	Reason  string
	Obs     WatchdogObservation
}

func (e *BackendBusyError) Error() string {
	return fmt.Sprintf("后端忙（%s）：%s", e.Verdict, e.Reason)
}

// eggCPUUsecFunc 造"读本单元 CPU 工时"的证据函数。
//
// 单元 cgroup 的解析口径与封闭性核验**同源**：`systemctl --user show <unit> -p ControlGroup`
// （需用户总线环境 ⇒ hatch 包里的 userScoped 已处理），再读 `/sys/fs/cgroup<cg>/cpu.stat`。
// 任何一步失败 ⇒ 返回 error ⇒ 看门狗走 degraded（回落固定超时），**绝不因缺读数误杀**。
func (m *Manager) eggCPUUsecFunc(sp *subproc) func() (uint64, error) {
	return func() (uint64, error) {
		if sp == nil {
			return 0, fmt.Errorf("没有卵实体")
		}
		if sp.Unit == "" {
			return 0, fmt.Errorf("该卵没有单元名（非孵化路径）⇒ 读不到单元工时")
		}
		h := m.hatcherImpl()
		uc, ok := h.(interface {
			UnitCgroup(context.Context, string) (string, error)
		})
		if !ok {
			return 0, fmt.Errorf("当前 hatcher 不支持读单元 cgroup")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cg, err := uc.UnitCgroup(ctx, sp.Unit)
		if err != nil {
			return 0, err
		}
		if cg == "" {
			return 0, fmt.Errorf("单元 %s 的 cgroup 路径为空", sp.Unit)
		}
		return eggCPUUsecAt(cg)
	}
}

// setEggWatchdog 把看门狗判词落到该卵的观测面（/eggs 读的就是它）。
// 阶段一（等头）与阶段二（正文停滞）都必须落 —— 不落 = 运维看不见，等于没判。
func (m *Manager) setEggWatchdog(sp *subproc, obs WatchdogObservation) {
	if sp == nil {
		return
	}
	m.mu.Lock()
	if cur, ok := m.procs[sp.model]; ok && cur == sp {
		o := obs
		cur.watchdog = &o
	}
	m.mu.Unlock()
}

// noteWatchdogStuck 判死动作：置 crashed（后续 acquireInflight 会拒绝新请求）+ 记账 + 日志。
// pin 未到期 ⇒ 只告警（Q7）；卵已被收走 ⇒ 无害返回。
func (m *Manager) noteWatchdogStuck(sp *subproc, verdict WatchdogVerdict, reason string) {
	if sp == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if cur, ok := m.procs[sp.model]; !ok || cur != sp {
		return
	}
	if pinned, _ := pinState(sp, time.Now()); pinned {
		log.Printf("[backend] ⚠ 看门狗判 %s，但该卵 pin 未到期 ⇒ 只告警不判死: model=%s %s",
			verdict, sp.model, reason)
		return
	}
	sp.state = StateCrashed
	sp.failCnt++
	log.Printf("[backend] 看门狗判死: model=%s verdict=%s unit=%s 理由=%s（置 crashed：不再派新请求；不收卵）",
		sp.model, verdict, sp.Unit, reason)
}

// asBackendBusy 取 Busy 错误的详情（供服务端写进响应体）。
func asBackendBusy(err error) (*BackendBusyError, bool) {
	var bbe *BackendBusyError
	if errors.As(err, &bbe) {
		return bbe, true
	}
	return nil, false
}

// ── ★ 判卡后的两级动作与「有界」封顶（J5 · `GAP-20261014-61`）────────────────────

// stopGraceFromEnv SIGTERM ⇒ SIGKILL 之间的宽限（**可配**：env `ZERG_STOP_GRACE_S`，默认 5 s）。
//
// ★ 这一条是 J5 三条牙之③「底下不许有不可配的硬 timeout」的落地：
// 任何一层硬窗口都要能**显式抬**（改 env）或**豁免**（置 0 走默认、或直接关看门狗一键回退），
// 且豁免要在观测面留痕（`WatchdogObservation.Waived` / `ZERG_WATCHDOG_WAIVE`）。
func stopGraceFromEnv() time.Duration {
	if v := strings.TrimSpace(watchdogEnv("ZERG_STOP_GRACE_S")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			return time.Duration(f * float64(time.Second))
		}
	}
	return 5 * time.Second
}

// recordStaleKill 记一次判卡（重连）并判**封顶**（J5「有界」那一牙）。返回 (已用次数, 是否已封顶)。
//
// 封顶后该卵置 `abandoned`：`ensureReady` 直接拒、✗ 不再重置熔断/重连 —— 否则"无上限的进度重置会变死循环"。
func (m *Manager) recordStaleKill(sp *subproc, max int) (int, bool) {
	if max <= 0 {
		max = DefaultMaxStaleKills
	}
	if sp == nil {
		return 0, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	sp.staleKills++
	if sp.staleKills > max {
		sp.abandoned = true
		sp.state = StateCrashed
		log.Printf("[backend] ⚠ 重连封顶用尽（max_stale_kills=%d · 已 %d 次）⇒ abandoned：不再重置/重连 "+
			"（✗ 无上限的进度重置会变死循环）model=%s", max, sp.staleKills, sp.model)
		return sp.staleKills, true
	}
	return sp.staleKills, false
}

// hardStopStalled 判卡**第二级**：硬断（SIGKILL 语义）。
//
// 走 `stopSubproc`（内部 SIGTERM ⇒ 宽限 `ZERG_STOP_GRACE_S` ⇒ SIGKILL）；宽限可配 ⇒ 不是"不可配的硬 timeout"。
func (m *Manager) hardStopStalled(sp *subproc) {
	if sp == nil {
		return
	}
	log.Printf("[backend] 看门狗两级动作·第二级：硬断（SIGKILL 语义）model=%s unit=%s", sp.model, sp.Unit)
	m.stopSubproc(sp)
}

// ---------------------------------------------------------------------------
// T12 进度证据（引擎自报进度）
// ---------------------------------------------------------------------------

// slotsProbe /slots 里我们**实测**用得到的字段（2026-09-15 生产卵长预填充期间取值）：
//
//	[{"id":2,"n_ctx":262144,"is_processing":true,"id_task":6,
//	  "n_prompt_tokens":198698,"n_prompt_tokens_processed":196650,"n_prompt_tokens_cache":0, ...}]
//
// 故主证据用 `n_prompt_tokens_processed`。**不许猜字段名**：这份结构是真机读出来的。
type slotsProbe struct {
	ID         int    `json:"id"`
	Processing bool   `json:"is_processing"`
	Processed  uint64 `json:"n_prompt_tokens_processed"`
	Total      uint64 `json:"n_prompt_tokens"`
}

// eggProgressFunc 读该卵引擎"已处理 token 数"：取所有正在处理槽的 processed 之和。
// 任何一步失败 ⇒ 返回错误（调用侧据此退化为"只用 CPU 工时"判定，不误杀）。
// ★★ 两个源（2026-10-16 · 2.5.15「D 进度源」· ✗ 注入点与签名不动）：
// ① 首选 `/slots`（上一行那句就是它：所有处理中槽的 processed 之和 · llama.cpp 系 · 逐字未变）；
// ② 回落**引擎自己的输出行** —— 该引擎**没有** `/slots` 时（DS4/DwarfStar 实测 404）⇒ 解析它
// stderr 里的进度行（`engine_progress.go` 的 `parseEngineProgress`）。
// ★ 回落只在 `/slots` 读到**非 200** 时发生；回落也读不到 ⇒ 仍按「读不到」返回 —— 与今天**等价**，
// 零回归、零误杀（读不到 ≠ 没推进 · `progress_guard.go` 的 fail-open 口径一字未动）。
func (m *Manager) eggProgressFunc(sp *subproc) func() (uint64, error) {
	port := sp.port
	return func() (uint64, error) {
		if port <= 0 {
			return 0, fmt.Errorf("进度不可读：端口未知")
		}
		// ★ 已缓存「这一枚引擎没有 /slots」⇒ 不再每次采样白打一次 404，直接走引擎输出行。
		if sp.unitProgress.slotsAbsent() {
			if v, ok := m.engineProgressFromEngine(sp); ok {
				return v, nil
			}
			return 0, fmt.Errorf("进度不可读：该引擎无 /slots（已缓存），且引擎输出行里没有进度")
		}
		cl := &http.Client{Timeout: 3 * time.Second}
		resp, err := cl.Get(fmt.Sprintf("http://127.0.0.1:%d/slots", port))
		if err != nil {
			return 0, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			// ★ 回落（2026-10-16 · 2.5.15「D 进度源」）：无 /slots 的引擎（DS4）——
			// 改读引擎自己的输出行；同时记下缓存，后续采样不再白打 404。
			sp.unitProgress.noteSlotsAbsent()
			if v, ok := m.engineProgressFromEngine(sp); ok {
				return v, nil
			}
			return 0, fmt.Errorf("进度不可读：/slots HTTP %d", resp.StatusCode)
		}
		raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if err != nil {
			return 0, err
		}
		var slots []slotsProbe
		if err := json.Unmarshal(raw, &slots); err != nil {
			return 0, fmt.Errorf("进度不可读：/slots 解析失败: %w", err)
		}
		var sum uint64
		for _, s := range slots {
			if s.Processing {
				sum += s.Processed
			}
		}
		return sum, nil
	}
}
