package backend

// 阶段二（T14）：正文停滞看门狗。
//
// 背景（A8 实测）：看门狗原先在"响应头到了"就退场，而流式下 llama-server **立刻**回响应头
// （预填充期只发 SSE 保活注释 `:`），于是长预填充只剩"固定总超时"这一堵墙 ⇒ 30 万 token 的
// 合法请求在第 5 分钟被掐断（回 200 + 9 条保活，既非答案也非报错）。
//
// 现在：看门狗**不在"头到了"退场**，继续守到"有推进"为止。
//
// 判据（只看证据不看表，Mr2109 2026-09-15 拍板"尽量不用时间限制"）：
//   · 有推进（引擎自报已处理 token 数在涨，或正文内容字节在涨） ⇒ 清空停滞计数，继续等；
//   · 无推进，且本单元 CPU 工时也不动（连续 DeadFlat 次采样） ⇒ **立刻判死**（提前，不必等窗口用满）；
//   · 无推进，但 CPU 在涨（"占着 CPU 在磨"） ⇒ 停滞窗口计数 +1，累计到 MaxStall ⇒ 判死；
//   · 进度读数不可用 ⇒ 退化为只用 CPU：涨 = 继续等，不动 = 按上面两条处理（仍收敛，不会永远等）。
//
// 实测数据（2026-09-15，生产卵 :9400，Qwen3.8-27B，198,698 token 的提示）：
// progress 0.87 时已耗时 1,566 s、125–136 tok/s、预计 ≈30 min；同期单元 CPU 5 s 涨 5,008,555 usec。
// ⇒ 合法长预填充本身就要半小时，所以**任何按秒判死都会被误杀**，判据只能是"有没有推进"。

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

	// content 已读到的**正文内容**字节数（排除 SSE 注释/保活行）。
	content atomic.Uint64
	lineBuf []byte
}

// newProgressGuard 包装 rc，并在后台跑阶段二状态机。
//
// progress：引擎自报进度读数（nil 或读失败 ⇒ 退化为只用 CPU 判据）。
// cpu：本单元累计 CPU 工时读数（微秒）。
// onStuck：判死动作（由调用方负责掐请求、记账；本函数只负责判）。
func newProgressGuard(
	rc io.ReadCloser,
	cfg WatchdogConfig,
	progress func() (uint64, error),
	cpu func() (uint64, error),
	onStuck func(verdict WatchdogVerdict, reason string),
) *progressGuard {
	g := &progressGuard{rc: rc, stop: make(chan struct{})}
	go g.run(cfg, progress, cpu, onStuck)
	return g
}

func (g *progressGuard) Read(p []byte) (int, error) {
	n, err := g.rc.Read(p)
	if n > 0 {
		g.countContent(p[:n])
	}
	return n, err
}

func (g *progressGuard) Close() error {
	g.once.Do(func() { close(g.stop) })
	return g.rc.Close()
}

// countContent 行感知计数：SSE 注释行（保活，形如 `:`）不算内容，其余字节都算。
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

// run 阶段二状态机：窗口 Window，窗口内每 Sample 采一次。
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
	maxStall := cfg.MaxWindows
	if maxStall <= 0 {
		maxStall = 5
	}
	deadFlat := cfg.DeadFlatSamples
	if deadFlat <= 0 {
		deadFlat = 2
	}

	lastProgress := g.progressValue(progress)
	progressReadable := false
	_ = progressReadable
	lastCPU, _ := cpu()
	flatCPU := 0
	stalls := 0

	deadline := time.NewTimer(window)
	defer deadline.Stop()
	tick := time.NewTicker(sample)
	defer tick.Stop()

	for {
		select {
		case <-g.stop:
			return // 正文读完/被关 ⇒ 守卫退场（正常路径）
		case <-tick.C:
			cur, readable := g.progressValue2(progress)
			curCPU, err := cpu()
			if cur > lastProgress {
				// 有推进 ⇒ 清空停滞计数（合法长 prefill 就靠这条活着）
				lastProgress = cur
				stalls, flatCPU = 0, 0
				continue
			}
			if err == nil && curCPU > lastCPU {
				flatCPU = 0 // 还在花力气 ⇒ 不按"彻底不动"处理
			} else {
				flatCPU++
				if flatCPU >= deadFlat {
					onStuck(WatchdogStuckCPUStalled, "正文阶段：无推进且本单元 CPU 工时不动（连续 "+itoa(flatCPU)+" 次采样）")
					return
				}
			}
			lastCPU = curCPU
			progressReadable = readable
		case <-deadline.C:
			curW, readableW := g.progressValue2(progress)
			advanced := curW > lastProgress
			// ⚠ 关键口径（沙箱批 D 抓到的假阳性）：**进度读数不可读时绝不能按窗口判死** ——
			// 读不到 ≠ 没推进；否则一次 /slots 读取失败就会把合法的长预填充（实测 ≈30 min 那种）杀掉。
			// 不可读时只保留"CPU 工时也不动"这条（那是真停摆的证据）。
			if advanced {
				lastProgress = curW
				stalls, flatCPU = 0, 0
			} else if !readableW && g.content.Load() == 0 {
				log.Printf("[看门狗] 正文阶段窗口 %d：进度不可读且无内容 ⇒ 只按 CPU 判（不累计停滞）", stalls)
			} else {
				stalls++
				log.Printf("[看门狗] 正文阶段窗口 %d/%d：无推进（进度可读=%v 内容=%dB CPU增量见上）",
					stalls, maxStall, readableW, g.content.Load())
				if stalls >= maxStall {
					onStuck(WatchdogStuckNoProgress, "正文阶段：无推进已累计 "+itoa(stalls)+" 个窗口（上限 "+itoa(maxStall)+"）")
					return
				}
			}
			progressReadable = readableW
			deadline.Reset(window)
		}
	}
}

// progressValue2 返回 (进度值, 引擎侧读数是否可读)。不可读 ≠ 没推进 —— 见窗口结算处注释。
func (g *progressGuard) progressValue2(progress func() (uint64, error)) (uint64, bool) {
	total := g.content.Load()
	readable := false
	if progress != nil {
		if v, err := progress(); err == nil {
			total += v
			readable = true
		}
	}
	return total, readable
}

func (g *progressGuard) progressValue(progress func() (uint64, error)) uint64 {
	total := g.content.Load()
	if progress != nil {
		if v, err := progress(); err == nil {
			total += v // 引擎自报进度 + 客户端已见内容：任一在涨都算推进
		}
	}
	return total
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
