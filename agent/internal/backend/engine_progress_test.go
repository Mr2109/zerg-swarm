package backend

// engine_progress_test.go —— 「D 进度源」（2.5.15）：无 `/slots` 引擎的只读进度信号判据。
//
// 设计真源：《DS4进度源-设计-20261016》§3.2 / §4.1 / §4.5。
//
// ★ fixture 用**实机逐字行**（`journalctl -o cat` 抄下；真行见设计件 §3.2(丙)），
// 不是凭空合成的形状：
//
//	bwrap[1404525]: 1015 17:53:44 ds4-server: listening on http://127.0.0.1:9400
//	bwrap[1404525]: 1015 17:53:45 ds4-server: chat ctx=0..5:5 prompt start
//	bwrap[1404525]: 1015 17:53:51 ds4-server: chat ctx=5..13:8 gen=8 THINKING decoding chunk=4.44 t/s avg=4.44 t/s 1.803s
//	bwrap[1404525]: 1015 17:53:51 ds4-server: chat ctx=0..14267:14267 TOOLS chunk 12288/14267 (86.1%) chunk=91.66 t/s
//
// 只有**计数**被替换的那几行是按同一打印格式串（`ds4_server.c` 的 `server_progress_cb` /
// `log_decode_progress`）现拼的：字段名、分隔符、单位与真行逐字同形，计数不同。
// ★ 本件只用**注入的假 runner**（`engineJournalRunner`）取数：不跑真 `journalctl`、不碰本机状态。

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// ── 实机逐字行（journalctl -o cat 抄下；尾行按真格式串换计数） ────────────────────

const (
	// ds4RealListening / ds4RealCtxBuf：**无进度**的两行（起服时必出，用来证明「读不到」）。
	ds4RealListening = `bwrap[1404525]: 1015 17:53:44 ds4-server: listening on http://127.0.0.1:9400`
	ds4RealCtxBuf    = `bwrap[1404525]: 1015 17:53:44 ds4-server: context buffers 11343.75 MiB (ctx=524288, backend=rocm, prefill_chunk=4096, raw_kv_rows=4352, compressed_kv_rows=131074)`
	// ds4RealPromptStart：请求纪元边界（真行）。
	ds4RealPromptStart = `bwrap[1404525]: 1015 17:53:45 ds4-server: chat ctx=0..5:5 prompt start`
	// ds4RealPromptDone：收尾行（真行）——**不是**纪元边界，不该重置任何东西。
	ds4RealPromptDone = `bwrap[1404525]: 1015 17:53:49 ds4-server: chat ctx=0..5:5 prompt done 3.539s`
	// ds4RealGen8：解码进度（真行）。
	ds4RealGen8 = `bwrap[1404525]: 1015 17:53:51 ds4-server: chat ctx=5..13:8 gen=8 THINKING decoding chunk=4.44 t/s avg=4.44 t/s 1.803s`
	// ds4RealChunk12288：预填充进度（真行）。
	ds4RealChunk12288 = `bwrap[1404525]: 1015 17:53:51 ds4-server: chat ctx=0..14267:14267 TOOLS chunk 12288/14267 (86.1%) chunk=91.66 t/s`
)

// ds4ChunkLine 预填充进度行（同真格式串：`ds4-server: %s ctx=%s chunk %d/%d (%.1f%%) chunk=%.2f t/s`）。
func ds4ChunkLine(cur, total int) string {
	pct := float64(cur) / float64(total) * 100
	return fmt.Sprintf("bwrap[1404525]: 1015 17:53:51 ds4-server: chat ctx=0..%d:%d TOOLS chunk %d/%d (%.1f%%) chunk=91.66 t/s",
		total, total, cur, total, pct)
}

// ds4GenLine 解码进度行（同真格式串：`ds4-server: %s ctx=%s gen=%d%s%s decoding chunk=%.2f t/s avg=%.2f t/s %.3fs`）。
func ds4GenLine(gen int) string {
	return fmt.Sprintf("bwrap[1404525]: 1015 17:53:52 ds4-server: chat ctx=5..%d:%d gen=%d THINKING decoding chunk=4.44 t/s avg=4.44 t/s %.3fs",
		5+gen, gen, gen, float64(gen)*0.26)
}

// ── 用例① 预填充计数严格递增 ────────────────────────────────────────────────

func TestParseEngineProgress_ChunkStrictlyIncreasing(t *testing.T) {
	first, ok1 := parseEngineProgress(ds4RealPromptStart + "\n" + ds4ChunkLine(4096, 132096))
	second, ok2 := parseEngineProgress(ds4RealPromptStart + "\n" + ds4ChunkLine(4096, 132096) + "\n" + ds4ChunkLine(8192, 132096))
	if !ok1 || !ok2 {
		t.Fatalf("有进度行却报读不到：ok1=%v ok2=%v", ok1, ok2)
	}
	if first != 4096 || second != 8192 {
		t.Fatalf("预填充计数不对：第一次=%d（期望 4096）· 第二次=%d（期望 8192）", first, second)
	}
	if second <= first {
		t.Fatalf("预填充计数未严格递增：%d → %d", first, second)
	}
}

// ── 用例② 跨相不回退（预填充 8192 → 解码 gen=50 / gen=100） ──────────────────

func TestParseEngineProgress_CrossPhaseNoRegression(t *testing.T) {
	tail50 := ds4RealPromptStart + "\n" + ds4ChunkLine(8192, 132096) + "\n" + ds4GenLine(50)
	got50, ok50 := parseEngineProgress(tail50)
	if !ok50 {
		t.Fatalf("跨相尾巴却报读不到：%q", tail50)
	}
	if got50 != 8192+50 { // 预填充冻结在 8192，解码累到 50 ⇒ 计数 = 8242
		t.Fatalf("跨相计数不对：%d（期望 %d＝预填充 8192 ＋ 解码 50）", got50, 8192+50)
	}
	got100, ok100 := parseEngineProgress(tail50 + "\n" + ds4GenLine(100))
	if !ok100 {
		t.Fatal("加一行 gen=100 后却报读不到")
	}
	if got100 != 8192+100 {
		t.Fatalf("跨相计数不对：%d（期望 %d）", got100, 8192+100)
	}
	if got100 <= got50 {
		t.Fatalf("跨相回退了：gen=50 时 %d → gen=100 时 %d", got50, got100)
	}
	// 同一条尾巴只涨解码：预填充那笔不许被重复累加。
	if got100-got50 != 50 {
		t.Fatalf("解码增量不是 50：%d（%d → %d）", got100-got50, got50, got100)
	}
}

// ── 用例③ 幂等（同输入同输出；轮询会反复覆盖同一段） ────────────────────────

func TestParseEngineProgress_Idempotent(t *testing.T) {
	tails := []string{
		ds4RealPromptStart + "\n" + ds4RealChunk12288,
		ds4RealPromptStart + "\n" + ds4ChunkLine(8192, 132096) + "\n" + ds4RealGen8,
		ds4RealListening + "\n" + ds4RealCtxBuf, // 读不到那一档也要幂等（(0,false) 逐字相同）
		"",
	}
	for i, tail := range tails {
		v1, ok1 := parseEngineProgress(tail)
		v2, ok2 := parseEngineProgress(tail)
		if v1 != v2 || ok1 != ok2 {
			t.Fatalf("第 %d 条尾巴两次读数不同：(%d,%v) vs (%d,%v)", i+1, v1, ok1, v2, ok2)
		}
	}
}

// ── 用例④ 无进度行 ⇒ (0,false)（✗ 不是 (0,true)） ───────────────────────────

func TestParseEngineProgress_NoProgressLines(t *testing.T) {
	cases := []struct{ name, tail string }{
		{"空串", ""},
		{"全空行", "\n\n\n"},
		{"只有起服两行（真行）", ds4RealListening + "\n" + ds4RealCtxBuf},
		{"只有 prompt done（真行）", ds4RealPromptDone}, // 收尾行不是纪元边界、也不是进度行
	}
	for _, c := range cases {
		if v, ok := parseEngineProgress(c.tail); ok || v != 0 {
			t.Fatalf("%s：得 (%d,%v)，期望 (0,false)（读不到 ≠ 没推进）", c.name, v, ok)
		}
	}
	// 另起一档：解码行里的 `chunk=4.44 t/s` **不许**被当成预填充的 `chunk cur/total`。
	// 该行只该靠 `gen=8` 命中 ⇒ 得 8（若被误读成预填充，会多出 4）。
	if v, ok := parseEngineProgress(ds4RealGen8); !ok || v != 8 {
		t.Fatalf("解码真行的 chunk=<浮点> 被误读：得 (%d,%v)，期望 (8,true)", v, ok)
	}
}

// ── 用例④b 只有 `prompt start` ⇒ (0,true)：纪元边界本身算「读得到」 ──────────

func TestParseEngineProgress_PromptStartIsReadableZero(t *testing.T) {
	// 口径：与 `/slots` 在「槽正在处理、processed 还是 0」时给 (0,nil) 同形。
	// 为什么必须给 true：预填充**卡在第一个量子之内**时日志里就只有这一行 ——
	// 若这里报「读不到」，DS4 又会退回「读不到 ⇒ 不按预算判卡」，早段无人收口。
	if v, ok := parseEngineProgress(ds4RealPromptStart); !ok || v != 0 {
		t.Fatalf("只有 prompt start：得 (%d,%v)，期望 (0,true)", v, ok)
	}
}

// ── 用例⑤ prompt start 后纪元重置（旧 gen 不留账） ──────────────────────────

func TestParseEngineProgress_EpochReset(t *testing.T) {
	tail := strings.Join([]string{
		ds4RealPromptStart,         // 请求 A 开始
		ds4ChunkLine(8192, 132096), // A 的预填充推到 8192
		ds4GenLine(100),            // A 的解码到 100
		ds4RealPromptStart,         // 请求 B 开始 ⇒ 纪元重置
		ds4ChunkLine(4096, 132096), // B 的预填充 4096
	}, "\n")
	got, ok := parseEngineProgress(tail)
	if !ok {
		t.Fatal("有 B 的进度行却报读不到")
	}
	if got != 4096 {
		t.Fatalf("纪元未重置：得 %d，期望 4096（B 自己的预填充）—— A 的 100 不许带过来（若带着会得 %d）",
			got, 4096+100)
	}
	// 重置后新一轮推进要接得上（不许被 A 的历史账压住）。
	if v, _ := parseEngineProgress(tail + "\n" + ds4ChunkLine(12288, 132096)); v != 12288 {
		t.Fatalf("重置后新一轮推进没接上：得 %d，期望 12288", v)
	}
}

// ── 用例⑥ 真行三条（父单给的逐字行）原样跑 ──────────────────────────────────

func TestParseEngineProgress_RealLines(t *testing.T) {
	// 只给解码真行 ⇒ 认 gen=8。
	if v, ok := parseEngineProgress(ds4RealGen8); !ok || v != 8 {
		t.Fatalf("解码真行：得 (%d,%v)，期望 (8,true)", v, ok)
	}
	// prompt start ＋ 预填充真行 ⇒ 认 12288。
	v1, ok1 := parseEngineProgress(ds4RealPromptStart + "\n" + ds4RealChunk12288)
	if !ok1 || v1 != 12288 {
		t.Fatalf("预填充真行：得 (%d,%v)，期望 (12288,true)", v1, ok1)
	}
	// 同一请求内再出解码真行（中间夹着 prompt done）⇒ 12288 ＋ 8 ＝ 12296，且不回退。
	tail := strings.Join([]string{ds4RealPromptStart, ds4RealChunk12288, ds4RealPromptDone, ds4RealGen8}, "\n")
	v2, ok2 := parseEngineProgress(tail)
	if !ok2 || v2 != 12296 {
		t.Fatalf("跨相真行：得 (%d,%v)，期望 (12296,true)", v2, ok2)
	}
	if v2 <= v1 {
		t.Fatalf("真行跨相回退：%d → %d", v1, v2)
	}
}

// ── 取数器：命令行逐字 + 读不到的各档 ───────────────────────────────────────

func TestUnitEngineProgress_JournalTailCommand(t *testing.T) {
	var got []string
	engineJournalRunner = func(timeout time.Duration, name string, args ...string) (string, error) {
		got = append([]string{name}, args...)
		return ds4RealPromptStart + "\n" + ds4RealChunk12288, nil
	}
	defer func() { engineJournalRunner = nil }()

	m := &Manager{}
	v, ok := m.unitEngineProgress(&subproc{model: "deepseek-v4-flash", Unit: "zerg-deepseek-v4-flash"})
	if !ok || v != 12288 {
		t.Fatalf("取数器读数：得 (%d,%v)，期望 (12288,true)", v, ok)
	}
	want := "journalctl --user -u zerg-deepseek-v4-flash -n 40 --no-pager -o cat"
	if joined := strings.Join(got, " "); joined != want {
		t.Fatalf("取数命令行不对：\n得   %s\n期望 %s", joined, want)
	}
}

func TestUnitEngineProgress_UnreadablePaths(t *testing.T) {
	m := &Manager{}
	calls := 0
	engineJournalRunner = func(timeout time.Duration, name string, args ...string) (string, error) {
		calls++
		return "", nil
	}
	defer func() { engineJournalRunner = nil }()

	// ① 单元名为空（裸 exec 路径 / 无单元名的卵）⇒ (0,false)，且**一次都不 fork**。
	if v, ok := m.unitEngineProgress(&subproc{model: "llama", Unit: ""}); ok || v != 0 {
		t.Fatalf("缺单元名：得 (%d,%v)，期望 (0,false)", v, ok)
	}
	if calls != 0 {
		t.Fatalf("缺单元名还是 fork 了 %d 次", calls)
	}
	// ② 命令失败且无输出 ⇒ (0,false)（fail-open：与今天等价）。
	engineJournalRunner = func(timeout time.Duration, name string, args ...string) (string, error) {
		return "", fmt.Errorf("journalctl: 用户总线不可达")
	}
	if v, ok := m.unitEngineProgress(&subproc{Unit: "zerg-deepseek-v4-flash"}); ok || v != 0 {
		t.Fatalf("命令失败：得 (%d,%v)，期望 (0,false)", v, ok)
	}
	// ③ 命令非零退出但带了输出（journalctl 的常见形态）⇒ 照读输出，不因 rc 丢掉证据。
	engineJournalRunner = func(timeout time.Duration, name string, args ...string) (string, error) {
		return ds4RealPromptStart + "\n" + ds4RealChunk12288, fmt.Errorf("rc=1")
	}
	if v, ok := m.unitEngineProgress(&subproc{Unit: "zerg-deepseek-v4-flash"}); !ok || v != 12288 {
		t.Fatalf("命令失败但有输出：得 (%d,%v)，期望 (12288,true)", v, ok)
	}
	// ④ 输出里没有进度行 ⇒ (0,false)。
	engineJournalRunner = func(timeout time.Duration, name string, args ...string) (string, error) {
		return ds4RealListening + "\n" + ds4RealCtxBuf, nil
	}
	if v, ok := m.unitEngineProgress(&subproc{Unit: "zerg-deepseek-v4-flash"}); ok || v != 0 {
		t.Fatalf("无进度行：得 (%d,%v)，期望 (0,false)", v, ok)
	}
}

// ── 状态件：3 s 节流 ＋「无 /slots」缓存（纯逻辑，不依赖真钟） ────────────────

func TestEngineProgressState_ThrottleAndSlotsCache(t *testing.T) {
	var st engineProgressState
	fetches := 0
	fetch := func() (uint64, bool) { fetches++; return 4096, true }
	base := time.Now()
	if v, ok := st.read(base, fetch); !ok || v != 4096 || fetches != 1 {
		t.Fatalf("首次读：得 (%d,%v) fetches=%d，期望 (4096,true) 1", v, ok, fetches)
	}
	if v, ok := st.read(base.Add(time.Second), fetch); !ok || v != 4096 || fetches != 1 {
		t.Fatalf("节流窗内该回缓存：得 (%d,%v) fetches=%d，期望 (4096,true) 1", v, ok, fetches)
	}
	if v, ok := st.read(base.Add(engineProgressPollEvery), fetch); !ok || v != 4096 || fetches != 2 {
		t.Fatalf("节流到期该真读：得 (%d,%v) fetches=%d，期望 (4096,true) 2", v, ok, fetches)
	}
	// 读不到那一档**不缓存**：下次仍真读（别把「读不到」冻成 3 s 的假值）。
	var st2 engineProgressState
	miss := func() (uint64, bool) { return 0, false }
	if _, ok := st2.read(base, miss); ok {
		t.Fatal("读不到却报 ok=true")
	}
	if _, ok := st2.read(base.Add(time.Second), miss); ok {
		t.Fatal("第二次仍该是读不到")
	}
	// 「无 /slots」缓存开关。
	if st.slotsAbsent() {
		t.Fatal("还没记过就报已有缓存")
	}
	st.noteSlotsAbsent()
	if !st.slotsAbsent() {
		t.Fatal("记过之后缓存没生效")
	}
}

// ── 接线：`/slots` 读不到 ⇒ 回落引擎输出行（＋缓存「无 /slots」＋ 3 s 节流） ────

func testServerPort(t *testing.T, srv *httptest.Server) int {
	t.Helper()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("解析测试服务地址失败：%v", err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("取端口失败：%v", err)
	}
	return port
}

func TestEggProgressFunc_FallsBackToEngineOutput(t *testing.T) {
	var httpHits, journalCalls int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&httpHits, 1)
		http.NotFound(w, r) // ★ DS4 现状：GET /slots ⇒ 404
	}))
	defer srv.Close()
	engineJournalRunner = func(timeout time.Duration, name string, args ...string) (string, error) {
		atomic.AddInt64(&journalCalls, 1)
		return ds4RealPromptStart + "\n" + ds4RealChunk12288, nil
	}
	defer func() { engineJournalRunner = nil }()
	port := testServerPort(t, srv)
	m := &Manager{}

	sp := &subproc{model: "deepseek-v4-flash", port: port, Unit: "zerg-deepseek-v4-flash"}
	fn := m.eggProgressFunc(sp)
	v1, err1 := fn()
	if err1 != nil {
		t.Fatalf("有引擎输出行却报错：%v", err1)
	}
	if v1 != 12288 {
		t.Fatalf("回落读数：得 %d，期望 12288", v1)
	}
	// 第二次（同一 sp）：①「无 /slots」已缓存 ⇒ 不再白打 404；② 3 s 节流内 ⇒ 不重复 fork。
	if v2, err2 := fn(); err2 != nil || v2 != 12288 {
		t.Fatalf("第二次读数：得 (%d,%v)，期望 (12288,nil)", v2, err2)
	}
	if got := atomic.LoadInt64(&httpHits); got != 1 {
		t.Fatalf("404 打了 %d 次，期望 1 次（第二次该走缓存）", got)
	}
	if got := atomic.LoadInt64(&journalCalls); got != 1 {
		t.Fatalf("journalctl 跑了 %d 次，期望 1 次（3 s 节流内回缓存）", got)
	}

	// 换一枚 sp（同端口、状态全新）：第一次仍会打一次 404 建立缓存，第二次必须不再打。
	sp2 := &subproc{model: "deepseek-v4-flash", port: port, Unit: "zerg-deepseek-v4-flash"}
	fn2 := m.eggProgressFunc(sp2)
	for i := 0; i < 2; i++ {
		if v, err := fn2(); err != nil || v != 12288 {
			t.Fatalf("第 %d 次读数：得 (%d,%v)，期望 (12288,nil)", i+1, v, err)
		}
	}
	if got := atomic.LoadInt64(&httpHits); got != 2 {
		t.Fatalf("404 共打了 %d 次，期望 2 次（每枚 sp 各一次建立缓存）", got)
	}
	if got := atomic.LoadInt64(&journalCalls); got != 2 {
		t.Fatalf("journalctl 共跑了 %d 次，期望 2 次（每枚 sp 第一次各一次）", got)
	}
}

func TestEggProgressFunc_SlotsStillPreferred(t *testing.T) {
	var journalCalls int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// 2026-09-15 生产卵长预填充期间的**实测**取值。
		fmt.Fprint(w, `[{"id":2,"n_ctx":262144,"is_processing":true,"id_task":6,"n_prompt_tokens":198698,"n_prompt_tokens_processed":196650,"n_prompt_tokens_cache":0}]`)
	}))
	defer srv.Close()
	engineJournalRunner = func(timeout time.Duration, name string, args ...string) (string, error) {
		atomic.AddInt64(&journalCalls, 1)
		return ds4RealPromptStart + "\n" + ds4RealChunk12288, nil
	}
	defer func() { engineJournalRunner = nil }()
	m := &Manager{}
	sp := &subproc{model: "llama", port: testServerPort(t, srv), Unit: "zerg-llama"}
	v, err := m.eggProgressFunc(sp)()
	if err != nil {
		t.Fatalf("/slots 正常却报错：%v", err)
	}
	if v != 196650 {
		t.Fatalf("/slots 读数变了：得 %d，期望 196650（首选源逐字未变）", v)
	}
	if got := atomic.LoadInt64(&journalCalls); got != 0 {
		t.Fatalf("有 /slots 时还去读了 %d 次引擎输出（回落只许在 /slots 读不到时发生）", got)
	}
}

func TestEggProgressFunc_PortUnknownUnchanged(t *testing.T) {
	m := &Manager{}
	// ★ 端口未知这一档**逐字不变**（不回落）：本单只动「/slots 读到非 200」那一条。
	sp := &subproc{model: "deepseek-v4-flash", port: 0, Unit: "zerg-deepseek-v4-flash"}
	if _, err := m.eggProgressFunc(sp)(); err == nil {
		t.Fatal("端口未知却给了读数（该档应逐字不变，仍报错）")
	}
}

// ── 负控：读到「无进度」时**必须** ok=false（旧态就是这条：对 DS4 一律读不到） ──

func TestParseEngineProgress_NegativeControlOnUnreadable(t *testing.T) {
	// 改前态＝没有本解析器 ⇒ 对 DS4 的任何输出都是「读不到」⇒ 本用例的期望值即改前行为：
	// 空输入 / 只有非进度行 必须回 (0,false)（调用侧据此 fail-open，**绝不**按 0 判卡）。
	if v, ok := parseEngineProgress(""); ok || v != 0 {
		t.Fatalf("空输入：得 (%d,%v)，期望 (0,false)", v, ok)
	}
	if v, ok := parseEngineProgress("\n\n\n"); ok || v != 0 {
		t.Fatalf("全空行：得 (%d,%v)，期望 (0,false)", v, ok)
	}
	if v, ok := parseEngineProgress(ds4RealListening + "\n" + ds4RealCtxBuf); ok || v != 0 {
		t.Fatalf("只有起服两行：得 (%d,%v)，期望 (0,false)", v, ok)
	}
}
