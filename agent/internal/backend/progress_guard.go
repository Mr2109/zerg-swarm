package backend

// progress_guard.go —— 阶段二（T14）：响应体上的**两把正交时钟**（liveness ＋ progress）。
//
// 背景（A8 实测）：看门狗原先在"响应头到了"就退场，而流式下 llama-server **立刻**回响应头
// （预填充期只发 SSE 保活注释 `:`），于是长预填充只剩"固定总超时"这一堵墙 ⇒ 30 万 token 的
// 合法请求在第 5 分钟被掐断（回 200 + 9 条保活，既非答案也非报错）。
//
// ★ 判据换形（`GAP-20261014-61` · 设计-虫族子代理-20261014 §七.3 J4/J5）：
//
//	liveness 时钟 —— **任何活动都续**：读到任何字节（含 SSE 保活注释）、本单元 CPU 工时在涨。
//	                 它只回答「连接/引擎还活着吗」，**✗ 绝不**回答「有没有推进」。
//	progress 时钟 —— **只由「真实 chunk 计数」续**：正文内容字节计数差分 ／ 引擎自报已处理
//	                 token 计数差分（两者都是**计数**）。✗ 时间戳不算，✗ 保活注释不算
//	                （它只是心跳，不是交付）。
//
//	**判卡只看 progress**（报告里两把锁都出）：CPU 在涨、保活不断，都救不回冻结的 progress。
//
//	预算 = `max(no_progress, est_ctx ÷ 实测 tok/s × 2)`；`no_progress: 0 ＝ 关`（只保留"彻底停摆"早杀）。
//
//	判卡后**两级**（J5）：先**请求优雅退** ⇒ grace（默认 30 s；批作业档 120–300 s）到期仍无进展才**硬断**。
//	重连封顶 `max_stale_kills=3` 由调用方（manager）记，超了走 `abandoned`。
//
// 实测数据（2026-09-15，生产卵 :9400，Qwen3.8-27B，198,698 token 的提示）：
// progress 0.87 时已耗时 1,566 s、125–136 tok/s、预计 ≈30 min；同期单元 CPU 5 s 涨 5,008,555 usec。
// ⇒ 合法长预填充本身就要半小时（132K 档 ≈1380 s），所以**任何按"无输出时长"判死都会被误杀**：
// 判据只能是「有没有真推进」（progress），而"算得慢"要由**按现读窗口/实测速率算出来的预算**兜着。

import (
	"io"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// progressGuard 包在响应体外面：只观察、不改动字节流；Close 时停掉后台守卫。
type progressGuard struct {
	rc   io.ReadCloser
	stop chan struct{}
	once sync.Once

	// content 已读到的**正文内容**字节数（排除 SSE 注释/保活行）—— progress 的真实 chunk 计数之一。
	content atomic.Uint64
	// heartbeatAt 已读到的**任何**字节（含保活注释）—— liveness 证据（用 atomic 存 UnixNano）。
	heartbeatAt atomic.Int64
	lineBuf     []byte
}

// newProgressGuard 包装 rc，并在后台跑阶段二状态机。
//
// progress：引擎自报进度读数（nil 或读失败 ⇒ **不按预算判卡**，但仍保留"彻底停摆"早杀）。
// cpu：本单元累计 CPU 工时读数（微秒）—— **liveness** 证据（✗ 不是 progress）。
// onStuck：判卡/两级动作的通告（由调用方负责掐请求、硬断、记账；本函数只负责判与定时）。
//
// ★ `onStuck` 会被调**两次**（判卡第一级 `stuck_no_progress` ⇒ grace 内没退 ⇒ 第二级 `stalled_hard`）；
// 两次之间若读数恢复推进或流结束（stop 关掉）⇒ **不调第二次**。
func newProgressGuard(
	rc io.ReadCloser,
	cfg WatchdogConfig,
	progress func() (uint64, error),
	cpu func() (uint64, error),
	onStuck func(verdict WatchdogVerdict, reason string),
) *progressGuard {
	g := &progressGuard{rc: rc, stop: make(chan struct{})}
	g.heartbeatAt.Store(time.Now().UnixNano())
	go g.run(cfg, progress, cpu, onStuck)
	return g
}

func (g *progressGuard) Read(p []byte) (int, error) {
	n, err := g.rc.Read(p)
	if n > 0 {
		// ★ 读到任何字节 ⇒ 续 **liveness**（保活注释也算"它还活着"）。
		g.heartbeatAt.Store(time.Now().UnixNano())
		g.countContent(p[:n])
	}
	if err != nil {
		// 正文读完（io.EOF，正常路径）或读失败 ⇒ 立刻让守卫退场。
		// 为什么必须在这里退场（2026-09-16 第十四轮 -race 实测）：守卫只认「有没有推进」，
		// 而**流结束之后「无推进」是必然的** —— 不退场就会在 EOF 之后的第 deadFlat 次采样
		// 把**已经正常完成**的请求判成 stuck_cpu_stalled（选中用例 TestPhase2_Progressing_NotJudged
		// 偶发红，实测 1/4~1/5；生产 Sample=5s/DeadFlat=2 时这个窗口约 10 s，等于给
		// 「读完之后还没 Close」的调用方埋一颗假判死的雷）。
		g.once.Do(func() { close(g.stop) })
	}
	return n, err
}

func (g *progressGuard) Close() error {
	g.once.Do(func() { close(g.stop) })
	return g.rc.Close()
}

// countContent 行感知计数：SSE 注释行（保活，形如 `:`）不算内容，其余字节都算。
//
// ★ 这里就是「**真实 chunk 计数**」的那把尺 —— 它数的是**交付出去的字节**，
// 与"时间过去多久"无关（✗ 时间戳不能续 progress）。
func (g *progressGuard) countContent(b []byte) {
	for len(b) > 0 {
		i := indexByte(b, '\n')
		if i < 0 {
			g.lineBuf = append(g.lineBuf, b...)
			// 超长行（非流式的整篇 JSON 无换行）：按内容计，避免堆内存无限涨
			if len(g.lineBuf) > 64*1024 {
				g.content.Add(uint64(len(g.lineBuf)))
				g.lineBuf = g.lineBuf[:0]
			}
			return
		}
		g.lineBuf = append(g.lineBuf, b[:i]...)
		line := strings.TrimSpace(string(g.lineBuf))
		if line != "" && !strings.HasPrefix(line, ":") {
			g.content.Add(uint64(len(line)))
		}
		g.lineBuf = g.lineBuf[:0]
		b = b[i+1:]
	}
}

func indexByte(b []byte, c byte) int {
	for i := range b {
		if b[i] == c {
			return i
		}
	}
	return -1
}

// run 阶段二状态机：两把正交时钟 ＋ 两级判卡。
func (g *progressGuard) run(
	cfg WatchdogConfig,
	progress func() (uint64, error),
	cpu func() (uint64, error),
	onStuck func(verdict WatchdogVerdict, reason string),
) {
	window := cfg.Window
	if window <= 0 {
		window = 90 * time.Second
	}
	sample := cfg.Sample
	if sample <= 0 {
		sample = 5 * time.Second
	}
	deadFlat := cfg.DeadFlatSamples
	if deadFlat <= 0 {
		deadFlat = 2
	}
	budget, budgetWhy := cfg.ProgressBudget()
	grace := cfg.Grace
	if grace <= 0 {
		grace = DefaultGrace
	}

	// 两把锁的高水位：只有**单调增量**才算续钟。
	lastContent := g.content.Load()
	lastEngine, engineReadable := progressValue(progress)
	lastHeartbeat := time.Now()
	lastProgress := time.Now()
	_, _ = lastContent, lastHeartbeat
	lastCPU, _ := cpu()

	flatBoth := 0
	deadline := time.NewTimer(window)
	defer deadline.Stop()
	tick := time.NewTicker(sample)
	defer tick.Stop()

	for {
		select {
		case <-g.stop:
			return // 正文读完/被关 ⇒ 守卫退场（正常路径）
		case <-tick.C:
			curCPU, cpuErr := cpu()
			// ① liveness：读到任何字节 / **CPU 工时在涨** ⇒ 续 liveness（**绝不**续 progress）。
			hb := time.Unix(0, g.heartbeatAt.Load())
			heartbeatAdv := hb.After(lastHeartbeat)
			if heartbeatAdv {
				lastHeartbeat = hb
			}
			if cpuErr == nil && curCPU > lastCPU {
				heartbeatAdv = true
				lastCPU = curCPU
			}
			// ② progress：只认真实 chunk 计数（内容字节差分 ∪ 引擎计数器差分）。
			content := g.content.Load()
			engineNow, engineOK := progressValue(progress)
			progressAdv := false
			if content > lastContent || (engineOK && engineNow > lastEngine) {
				progressAdv = true
				if content > lastContent {
					lastContent = content
				}
				if engineOK && engineNow > lastEngine {
					lastEngine = engineNow
				}
				lastProgress = time.Now()
			}
			if engineOK {
				engineReadable = true
			}
			// 进度读数是否"可读"：只要有**任一**真实源给过数（内容出过 或 引擎计数器读到过），
			// 就说明"冻结"是真冻结；两者都没有 ⇒ 读不到 ⇒ **绝不按预算判卡**（读不到 ≠ 没推进）。
			progressReadable := engineReadable || lastContent > 0

			if progressAdv {
				flatBoth = 0
				continue
			}
			// ③ 真停摆（提前判死）：两把钟**双双冻结**。
			if !heartbeatAdv {
				flatBoth++
				if flatBoth >= deadFlat {
					log.Printf("[看门狗] 正文阶段：连续 %d 次采样两把钟双双冻结（content=%dB 引擎读数=%d）⇒ 提前判死",
						flatBoth, content, engineNow)
					onStuck(WatchdogStuckCPUStalled, "正文阶段：liveness 与 progress 双双冻结（连续 "+itoa(flatBoth)+" 次采样）")
					return
				}
			} else {
				flatBoth = 0
			}
			// ④ 判卡**只看 progress**：冻结超过有效预算 ⇒ 判卡（两级）。
			if budget > 0 && progressReadable {
				el := time.Since(lastProgress)
				if el >= budget {
					log.Printf("[看门狗] 正文阶段：progress 冻结 %s ≥ 预算 %s（%s）⇒ 判卡",
						el.Round(time.Second), budget.Round(time.Second), budgetWhy)
					// 第一级：请求**优雅退**（由调用方掐请求、让引擎自己收尾）。
					onStuck(WatchdogStuckNoProgress, "正文阶段：progress 冻结 "+el.Round(time.Second).String()+
						" ≥ 预算 "+budget.Round(time.Second).String()+"（第一级：请求优雅退，grace="+grace.Round(time.Second).String()+"）")
					// grace：它在这段时间里自己退了/流断了 ⇒ 收手，**不硬断**。
					select {
					case <-g.stop:
						return
					case <-time.After(grace):
					}
					// 第二级：**硬断**。
					onStuck(WatchdogStalledHard, "正文阶段：grace（"+grace.Round(time.Second).String()+
						"）到期仍无进展 ⇒ 硬断（SIGKILL 语义）")
					return
				}
			}
		case <-deadline.C:
			// 窗口只作**结算节奏**：真正的判卡在上面那条预算里（✗ 窗口本身不再是硬上界）。
			deadline.Reset(window)
		}
	}
}

// progressValue 取一次引擎侧进度读数（真实 chunk 计数之一）。读不到 ⇒ (0, false)。
func progressValue(progress func() (uint64, error)) (uint64, bool) {
	if progress == nil {
		return 0, false
	}
	v, err := progress()
	if err != nil {
		return 0, false
	}
	return v, true
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
