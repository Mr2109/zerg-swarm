// cli_core_load_test.go —— `zerg core load`（缺口 `GAP-20261021-03` · 主控负载可见面）的判据机检。
//
// 判据（逐条落成断言）：
//
//	① **判读有尺子**：`coreLoadLevel` 按**本机核数**折算 —— 897.5%（事故时读数）在 10 核机上 = `满载`，
//	   同一数字在 32 核机上 = `忙`；采不到（`ok=false`）⇒ `未知`，**绝不**落 `空闲`。
//	② **面能出**：合成主控（httptest · 不碰 8580）＋ 换掉的 `ps` ⇒ rc=0，表头齐、逐枚在装任务一行，
//	   `model / machine / phase / elapsed_s` 逐格非空（「在算什么 / 为谁算 / 已经多久」三问各有一格）。
//	③ **还要多久**：带 `expand` 进度的任务给出 `eta_s`（按已完成字节 ÷ 已耗时外推）；**没有进度读数
//	   的任务整格不给**（✗ 不编一个 0 冒充「马上好」）。
//	④ **机读面**：`--json` 六键包封 · `kind=CoreLoad` · 在册字段点得出来；未登记字段 ⇒ rc=2 且**零 stdout**。
//	⑤ **读不到不许当绿**：主控打不到 ⇒ rc=8 且 stdout 0 字节（不拿「没有」当健康）。
//
// 夹具口径：`ZERG_PORT` 指合成 server（同 `cli_matrix_test.go`）；`coreLoadPSCmd` / `coreLoadSleep`
// 是本件留的**唯一**采样口 ⇒ 单测不读本机真进程、不真睡。
package main

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// coreLoadStatusPayload —— 合成 `/api/core/status`：两枚任务（一枚在装 + 带 expand 进度 · 一枚已结算）。
const coreLoadStatusPayload = `{"ok":true,"pid":41453,"started_at":"2026-10-21T12:31:11+08:00","version":"2.5.15",` +
	`"load_tasks":[` +
	`{"task_id":"load-1-ms02-Egg-A","machine":"ms02","model":"Egg-A","phase":"accepted","started_at":"2026-10-21T12:31:20+08:00","elapsed_s":100,` +
	`"backend":"llama-server","port":9400,"expand":{"state":"expanding","bytes_done":2147483648,"bytes_total":8589934592,"segments_done":1,"segments_total":4,"elapsed_s":100}},` +
	`{"task_id":"load-2-ms03-Egg-B","machine":"ms03","model":"Egg-B","phase":"ok","started_at":"2026-10-21T11:00:00+08:00","elapsed_s":42,"finished_at":"2026-10-21T11:00:42+08:00"}` +
	`]}`

// coreLoadServer 起合成主控并把它接到 `ZERG_PORT`（不碰 8580 · 用完即关）。
func coreLoadServer(t *testing.T, payload string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/core/status" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	t.Cleanup(srv.Close)
	port := srv.Listener.Addr().(*net.TCPAddr).Port
	t.Setenv("ZERG_PORT", strconv.Itoa(port))
}

// coreLoadFakePS 换掉采样口：两帧之间累计 CPU 时间涨 2.00 秒（⇒ 瞬时占用读数必然 > 0）。
// 返回还原函数。
func coreLoadFakePS(t *testing.T) {
	t.Helper()
	origPS, origSleep := coreLoadPSCmd, coreLoadSleep
	coreLoadSleep = func(_ time.Duration) {} // 不真睡
	frames := []string{
		"  10:00.00  33:01:02  1048576\n", // 累计 CPU 10:00 · 已运行 33h01m02s · 常驻 1 GiB
		"  10:02.00  33:01:03  1048576\n", // 再跑 1 秒墙钟 ⇒ 2 秒 CPU ⇒ 约 200%
	}
	n := 0
	coreLoadPSCmd = func(args ...string) (string, error) {
		f := frames[len(frames)-1]
		if n < len(frames) {
			f = frames[n]
		}
		n++
		return f, nil
	}
	t.Cleanup(func() { coreLoadPSCmd, coreLoadSleep = origPS, origSleep })
}

// runCoreLoad —— 进程内跑本命令（不经命令树：登记行尚未落 main.go 时判据也要跑得起来）。
func runCoreLoad(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	inv, err := parseInvocation(argv)
	if err != nil {
		t.Fatalf("parseInvocation(%v) 失败：%v", argv, err)
	}
	inv.path = []string{"core", "load"}
	var out, errb bytes.Buffer
	rc := cmdCoreLoad(inv, &out, &errb)
	return rc, out.String(), errb.String()
}

// 判据 ① —— 判读尺子：按本机核数折算 · 采不到 ⇒ 未知（不是空闲）。
func TestCoreLoadLevelJudgment(t *testing.T) {
	cases := []struct {
		name   string
		cpuPct float64
		ncpu   int
		ok     bool
		want   string
	}{
		{"事故读数 897.5% 在 10 核机 = 满载", 897.5, 10, true, "满载"},
		{"同一读数 897.5% 在 32 核机 = 忙", 897.5, 32, true, "忙"},
		{"0% = 空闲", 0, 10, true, "空闲"},
		{"799.9% 在 10 核机（差一点点到满档）= 忙", 799.9, 10, true, "忙"},
		{"800% 在 10 核机（正好满档）= 满载", 800, 10, true, "满载"},
		{"采不到 ⇒ 未知（✗ 不落空闲）", 0, 10, false, "未知"},
		{"拿不到核数 ⇒ 未知（不编分母）", 500, 0, true, "未知"},
	}
	for _, c := range cases {
		if got := coreLoadLevel(c.cpuPct, c.ncpu, c.ok); got != c.want {
			t.Errorf("%s：coreLoadLevel(%.1f,%d,%t) = %q（要 %q）", c.name, c.cpuPct, c.ncpu, c.ok, got, c.want)
		}
	}
}

// 判据 ①（采样口径）—— 分母必须是**两帧之间的墙钟窗口**，不是「第二次 `ps` 的耗时」。
//
// 回归牙：本件写坏过一次 —— `t1` 取自 sleep **之后** ⇒ 分母只剩几毫秒、占用被放大百倍以上。
// 这里用「真睡 300ms + 涨 1.5 秒 CPU」钉住 `cpu_pct ≈ 500`（放大 100 倍的实现会给出 ~37500）。
func TestCoreLoadSampleWindow(t *testing.T) {
	origPS, origSleep := coreLoadPSCmd, coreLoadSleep
	t.Cleanup(func() { coreLoadPSCmd, coreLoadSleep = origPS, origSleep })
	coreLoadSleep = func(d time.Duration) { time.Sleep(300 * time.Millisecond) }
	frames := []string{"  10:00.00  01:00:00  2048\n", "  10:01.50  01:00:00  2048\n"}
	n := 0
	coreLoadPSCmd = func(args ...string) (string, error) {
		f := frames[len(frames)-1]
		if n < len(frames) {
			f = frames[n]
		}
		n++
		return f, nil
	}
	s := sampleCoreProc(41453)
	if !s.OK {
		t.Fatalf("采样失败：%s", s.Detail)
	}
	if s.CpuPct < 400 || s.CpuPct > 600 {
		t.Fatalf("cpu_pct = %.1f（要 ≈500 = 1.5s CPU ÷ 0.3s 窗口）· 窗口 %.3fs", s.CpuPct, s.WindowS)
	}
	if s.UptimeS != 3600 || s.RssMB != 2 {
		t.Errorf("uptime/rss = %.0fs / %.0fMB（要 3600 / 2）", s.UptimeS, s.RssMB)
	}
}

// 判据 ②③ —— 面能出：rc=0 · 表头齐 · 逐枚任务一行 · 「在算什么/为谁算/已经多久/还要多久」各有格。
func TestCoreLoadFaceRows(t *testing.T) {
	coreLoadServer(t, coreLoadStatusPayload)
	coreLoadFakePS(t)

	rc, out, errb := runCoreLoad(t)
	if rc != 0 {
		t.Fatalf("rc=%d（要 0）· stderr=%s", rc, errb)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 { // 表头 + 2 枚任务
		t.Fatalf("行数 %d（要 3 = 表头 + 2 枚任务）· 输出：\n%s", len(lines), out)
	}
	if got := strings.Split(lines[0], "\t"); strings.Join(got, ",") != strings.Join(coreLoadDisplay, ",") {
		t.Fatalf("表头 = %v（要 %v）", got, coreLoadDisplay)
	}
	// 第一枚（在装 · 带 expand 进度）
	a := strings.Split(lines[1], "\t")
	if a[0] != "满载" && a[0] != "忙" && a[0] != "空闲" {
		t.Fatalf("判读格 = %q（只许 满载/忙/空闲 —— 采样成功就不许是「未知」）", a[0])
	}
	if v, err := strconv.ParseFloat(a[1], 64); err != nil || v <= 0 {
		t.Errorf("cpu_pct 格 = %q（要 > 0 的数 —— 两帧差分涨了 2 秒 CPU）", a[1])
	}
	if a[4] != "Egg-A" || a[5] != "ms02" || a[6] != "accepted" || a[7] != "100" {
		t.Errorf("「在算什么/为谁算/到哪一步/多久」四格不齐：model=%q machine=%q phase=%q elapsed_s=%q", a[4], a[5], a[6], a[7])
	}
	// 「还要多久」：2 GiB 用了 100s ⇒ 剩 6 GiB ⇒ 约 300s
	if a[8] != "300" {
		t.Errorf("eta_s 格 = %q（要 300 = (8-2)GiB ÷ (2GiB/100s)）", a[8])
	}
	// 第二枚（已结算 · 无 expand）⇒ eta_s 整格不给（空串）
	b := strings.Split(lines[2], "\t")
	if len(b) != len(coreLoadDisplay) {
		t.Fatalf("第二行列数 %d（要 %d）：%q", len(b), len(coreLoadDisplay), lines[2])
	}
	if b[6] != "ok" {
		t.Errorf("第二枚 phase = %q（要 ok）", b[6])
	}
	if b[8] != "" {
		t.Errorf("无进度的任务 eta_s = %q（要整格不给 —— 不编一个数）", b[8])
	}
	if !strings.Contains(errb, "只读") {
		t.Errorf("stderr 没写明只读口径：%s", errb)
	}
}

// 判据 ④ —— 机读面：六键包封 · kind=CoreLoad · 在册字段点得出来 · 无进度的任务整键略去 eta_s。
func TestCoreLoadJSONFace(t *testing.T) {
	coreLoadServer(t, coreLoadStatusPayload)
	coreLoadFakePS(t)

	rc, out, errb := runCoreLoad(t, "--json", "task_id,level,cpu_pct,inflight,eta_s")
	if rc != 0 {
		t.Fatalf("rc=%d（要 0）· stderr=%s", rc, errb)
	}
	var env struct {
		Schema string           `json:"schema"`
		Kind   string           `json:"kind"`
		Items  []map[string]any `json:"items"`
		Meta   map[string]any   `json:"meta"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("包封不是 JSON：%v · 原样：%s", err, out)
	}
	if env.Schema != "zerg/v1" || env.Kind != "CoreLoad" {
		t.Fatalf("schema/kind = %q/%q（要 zerg/v1 · CoreLoad）", env.Schema, env.Kind)
	}
	if len(env.Items) != 2 {
		t.Fatalf("items 条数 %d（要 2）· 原样：%s", len(env.Items), out)
	}
	if _, ok := env.Items[0]["eta_s"]; !ok {
		t.Errorf("在装那枚缺 eta_s（expand 进度在 ⇒ 应该算得出来）：%v", env.Items[0])
	}
	if _, ok := env.Items[1]["eta_s"]; ok {
		t.Errorf("已结算那枚不该有 eta_s（无进度读数 ⇒ 整键略去）：%v", env.Items[1])
	}
	if env.Items[0]["inflight"] != "1" {
		t.Errorf("inflight = %v（要 \"1\" —— 两枚里只有一枚 accepted）", env.Items[0]["inflight"])
	}
}

// 判据 ④（负控）—— 未登记字段 ⇒ rc=2 · stdout 0 字节 · stderr 逐字点名 + 列合法字段。
func TestCoreLoadBadFieldZeroStdout(t *testing.T) {
	coreLoadServer(t, coreLoadStatusPayload)
	coreLoadFakePS(t)

	rc, out, errb := runCoreLoad(t, "--json", "cpu_usage")
	if rc != 2 {
		t.Fatalf("rc=%d（要 2）· stderr=%s", rc, errb)
	}
	if out != "" {
		t.Fatalf("用法错却写了 stdout %d 字节：%q", len(out), out)
	}
	if !strings.Contains(errb, `未知字段 "cpu_usage"`) || !strings.Contains(errb, "合法字段: level") {
		t.Fatalf("stderr 没逐字点名或没列合法字段：%s", errb)
	}
}

// 判据 ⑤ —— 主控打不到 ⇒ rc=8 · stdout 0 字节（「没有」不许当健康）。
func TestCoreLoadUnreachable(t *testing.T) {
	coreLoadServer(t, coreLoadStatusPayload)
	t.Setenv("ZERG_PORT", "1") // 没人听
	coreLoadFakePS(t)

	rc, out, errb := runCoreLoad(t)
	if rc != 12 {
		t.Fatalf("rc=%d（要 12 = unreachable · 打不到主控不许当健康）· stderr=%s", rc, errb)
	}
	if out != "" {
		t.Fatalf("拿不到结论却写了 stdout：%q", out)
	}
}
