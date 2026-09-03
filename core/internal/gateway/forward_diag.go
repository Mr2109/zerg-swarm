// package gateway — 转发失败时的进程内拨号诊断（2026-09-14）
//
// 背景：本机出现"只有主控进程拨不通 X3 agent（<worker-ip>:8100: no route to host），
// 而同机其它进程（终端 curl / launchd 作业 / 甚至以完全相同签名身份运行的探针）
// 都能拨通"的现象。为定位到具体层级，在主控自身进程内做三层对照：
//
//	① net.DialTimeout —— 自定义 http.Transport（DialContext 未设置）实际走的路径
//	② net.Dialer{}     —— http.DefaultTransport 走的路径
//	③ 本进程可见的接口地址 + 名字解析结果 —— 排除"路由/源地址/DNS"类原因
//
// 只读诊断，不改变任何转发行为；出错路径才触发。
package gateway

import (
	"log"
	"net"
	"net/url"
	"strings"
	"time"
)

// StartupDialDiagnostics 启动期拨号诊断（2026-09-14 临时排查用）。
// 由环境变量 ZERG_DIAG_STARTUP_DIAL 触发，值为逗号分隔的 host:port 列表。
// 挂在进程"出生时刻"（main 最早处），用来区分：
//   - 启动即失败 ⇒ 二进制静态属性/策略（从出生就被拒）
//   - 启动能通、运行期才失败 ⇒ 运行时状态（连接池/路由 socket）
func StartupDialDiagnostics(targets string) {
	log.Printf("🩺 [diag] 启动期拨号诊断开始（%s）", targets)
	for _, t := range strings.Split(targets, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			dialReport("启动期", t)
		}
	}
	log.Printf("🩺 [diag] 启动期拨号诊断结束")
}

// dialReport 对单个 host:port 做 ①②③ 三项对照并打日志（转发失败路径与启动期共用）。
func dialReport(tag, host string) {
	// ① 裸 net.DialTimeout（自定义 Transport 的默认路径）
	t0 := time.Now()
	if c, e := net.DialTimeout("tcp", host, 6*time.Second); e != nil {
		log.Printf("🩺 [diag] %s ① net.DialTimeout  ✗ %v （耗时 %s）", tag, e, time.Since(t0).Round(time.Millisecond))
	} else {
		log.Printf("🩺 [diag] %s ① net.DialTimeout  ✓ 已连接（耗时 %s）", tag, time.Since(t0).Round(time.Millisecond))
		c.Close()
	}

	// ② net.Dialer{}（默认 Transport 的路径）
	t1 := time.Now()
	d := net.Dialer{Timeout: 6 * time.Second}
	if c, e := d.Dial("tcp", host); e != nil {
		log.Printf("🩺 [diag] %s ② net.Dialer{}     ✗ %v （耗时 %s）", tag, e, time.Since(t1).Round(time.Millisecond))
	} else {
		log.Printf("🩺 [diag] %s ② net.Dialer{}     ✓ 已连接（耗时 %s）", tag, time.Since(t1).Round(time.Millisecond))
		c.Close()
	}

	// ③ 本进程可见的非回环地址
	if addrs, e := net.InterfaceAddrs(); e == nil {
		for _, a := range addrs {
			if ipn, ok := a.(*net.IPNet); ok && ipn.IP.To4() != nil && !ipn.IP.IsLoopback() {
				log.Printf("🩺 [diag] %s ③ 本进程可见地址 %s", tag, ipn.String())
			}
		}
	}
}

// logDialDiagnostics 在主控自身进程内做三层拨号对照并打日志（转发失败时触发）。
func (g *Gateway) logDialDiagnostics(rawURL string, origErr error) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		log.Printf("🩺 [diag] 无法解析转发 URL %q: %v", rawURL, err)
		return
	}
	log.Printf("🩺 [diag] 转发失败 → 进程内四层对照 host=%s 原错误=%v", u.Host, origErr)
	dialReport("转发期", u.Host)

	// ④ 名字解析（IP 字面量时给出解析结果以排除解析层）
	if ips, e := net.LookupHost(u.Hostname()); e != nil {
		log.Printf("🩺 [diag] ④ LookupHost(%s) ✗ %v", u.Hostname(), e)
	} else {
		log.Printf("🩺 [diag] ④ LookupHost(%s) = %v", u.Hostname(), ips)
	}
}
