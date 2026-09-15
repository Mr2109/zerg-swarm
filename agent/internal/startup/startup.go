// Package startup 让**程序自己**报告"启动到哪一步了/好没好"。
//
// 为什么要它（2026-09-16 实证，站立铁律"判到没到一律读程序自己的信号，禁止等 N 秒再看"）：
//   - 外部守卫睡 7 秒或轮询 60 秒去读 systemd 的 `activating`，既会把**好的改动误判成坏的**
//     （同一天把同一个好改动自动还原了两次），又在真卡住时**看不出卡在哪一步**；
//   - 同族的四次教训：90 s 首字节 / 5 min 总超时 / 脚本 `sleep N` 后判 go / 拿 `activating` 当判据。
//
// 口径：
//   - 启动过程中每一步都 Mark(阶段名) ⇒ 进日志 + 进内存快照；
//   - 全部完成后 Ready() ⇒ 日志 + `sd_notify(READY=1)`（unit 若是 Type=notify 即生效；不是也无害）；
//   - 对外通过 `/status.startup` 与 `/ready` 暴露 ⇒ 守卫**读信号或等事件**，不读秒。
package startup

import (
	"log"
	"net"
	"os"
	"sync"
	"time"
)

// Phase 一个启动阶段（名称 + 发生的时刻 + 距进程启动的毫秒数）。
type Phase struct {
	Name   string `json:"name"`
	AtMS   int64  `json:"at_ms"`   // 距进程启动的毫秒
	UnixMS int64  `json:"unix_ms"` // 绝对时刻（便于跨进程对表）
}

var (
	mu        sync.Mutex
	startedAt = time.Now()
	phases    []Phase
	ready     bool
	readyAtMS int64
)

// Mark 记录一个启动阶段（进日志 + 进快照）。重复调用同一名称会被记为多次（如实反映），
// 但正常启动路径每个阶段只应出现一次 —— 次数异常本身就是线索。
func Mark(name string) {
	ms := time.Since(startedAt).Milliseconds()
	mu.Lock()
	phases = append(phases, Phase{Name: name, AtMS: ms, UnixMS: time.Now().UnixMilli()})
	mu.Unlock()
	log.Printf("[启动阶段] +%dms %s", ms, name)
}

// Ready 标记"启动完成，可以服务了"：打日志 + 尽力 sd_notify(READY=1)。
// 幂等：重复调用只记一次。
func Ready() {
	mu.Lock()
	if ready {
		mu.Unlock()
		return
	}
	ready = true
	readyAtMS = time.Since(startedAt).Milliseconds()
	mu.Unlock()
	log.Printf("[启动阶段] +%dms 就绪（READY）", readyAtMS)
	notifyReady()
}

// Snapshot 读当前快照：当前阶段（最后 Mark 的）、是否就绪、距启动毫秒、阶段流水。
func Snapshot() (phase string, isReady bool, elapsedMS int64, history []Phase) {
	mu.Lock()
	defer mu.Unlock()
	if n := len(phases); n > 0 {
		phase = phases[n-1].Name
	}
	h := make([]Phase, len(phases))
	copy(h, phases)
	return phase, ready, time.Since(startedAt).Milliseconds(), h
}

// notifyReady 向 $NOTIFY_SOCKET 写 READY=1（systemd Type=notify 用）。
// 环境变量缺失或写入失败都**不是错误**：unit 大概率是 Type=simple，此时无需通知。
func notifyReady() {
	sock := os.Getenv("NOTIFY_SOCKET")
	if sock == "" {
		return
	}
	// 抽象命名空间：systemd 传入以 @ 开头，需换成 NUL 前缀
	if sock[0] == '@' {
		sock = "\x00" + sock[1:]
	}
	conn, err := net.Dial("unixgram", sock)
	if err != nil {
		log.Printf("[启动阶段] sd_notify 跳过（连不上 NOTIFY_SOCKET：%v）", err)
		return
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("READY=1")); err != nil {
		log.Printf("[启动阶段] sd_notify 写入失败：%v", err)
	}
}
