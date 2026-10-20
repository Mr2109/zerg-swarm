// orchestrate_test.go — 多服务编排七格（v3 §D ＋ 判据 12 增补）：
//
//	① TestOrchestrateSingleServiceStartStop      单服务起/停；Started/Log/Stops/States 都落地
//	② TestOrchestrateOrderEdgeBlocksOnDep        db→api 拓扑序：api 的 spawn 必须晚于 db 探到 200；
//	                                             反向再验一次——db 永不就绪 ⇒ api 一次都没起
//	③ TestOrchestrateCascadeStopOnDependencyDeath ★ 最关键那格：db 半路死（rc=1）⇒ api 被级联停，
//	                                             日志逐条写「因为 db 已退出（rc=1）」
//	④ TestOrchestrateReadinessTimeoutRollsBack   就绪超时 ⇒ 本次已起的按启动**逆序**停掉、
//	                                             错误里点名服务、实例被 Destroy
//	⑤ TestOrchestrateRejectsBadGraphDefensively  环依赖/悬空依赖/非法 Order/重名 —— 且都发生在
//	                                             Provision **之前**（be.Calls() 为空）
//	⑥ TestOrchestrateStopIdempotent              Stop 幂等（含并发两次 Stop 只停一遍）
//	⑦ TestOrchestrateStopLogsBecauseWhom         逆序停 + 每条日志写「因为谁」
//	⑧ TestOrchestrateDefaultSpawnerUsesBackend   缺省 Spawner 真走 FakeBackend.Exec（argv/Env/Dir 透传；
//	                                             rc=0 ⇒ compose completed，不触发存活边）
//	⑨ TestOrchestrateEmptyPathProbesTCP          readiness 无 path ⇒ 端口可连即就绪（不假装要 200）
//
// ★ 全程只用 FakeBackend 的 Provision/Expose/Destroy ＋ 一个可控 Proc 桩演「长驻/半路死」；
// 就绪探测打 httptest（回环），不碰真后端、不出网。
package cocoon

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ── 夹具：契约 ────────────────────────────────────────────────────────

// orchTOML — 把「服务段」拼进一份最小可过契约（真解析，不手搓 Spec）。
func orchTOML(services string) string {
	return `[茧]
name    = "shop"
image   = "python:3.12-slim"
digest  = "` + specValidDigest + `"
workdir = "/app"

` + services + `

[资源]
cpu     = 1
mem_gb  = 2
disk_gb = 10
`
}

func mustOrchSpec(t *testing.T, services string) *Spec {
	t.Helper()
	sp, err := ParseSpec([]byte(orchTOML(services)))
	if err != nil {
		t.Fatalf("夹具契约应当解析通过，得到错误: %v", err)
	}
	return sp
}

// 服务段常量（readiness 的 path 都以 / 开头——解析器只收绝对路径）。
const (
	svcDB = `[[服务]]
name      = "db"
entry     = ["postgres", "-D", "/var/lib/pg"]
readiness = { port = 5432, path = "/healthz", timeout = "2s" }
`
	svcAPIDepDB = `[[服务]]
name       = "api"
entry      = ["uvicorn", "app:api"]
depends_on = ["db"]
readiness  = { port = 8000, path = "/healthz", timeout = "2s" }
`
	svcWebDepAPI = `[[服务]]
name       = "web"
entry      = ["nginx", "-g", "daemon off;"]
depends_on = ["api"]
`
)

// ── 夹具：可控进程（演长驻 / 半路死 / 被停）────────────────────────────

// orchProc — 测试用进程：Wait 阻塞到 exit() 被调；Stop 记账并自报 SIGTERM。
type orchProc struct {
	name  string
	box   *orchBox
	done  chan struct{}
	once  sync.Once
	mu    sync.Mutex
	res   ExecResult
	stops atomic.Int32
}

func (p *orchProc) Wait() ExecResult {
	<-p.done
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.res
}

func (p *orchProc) Stop(ctx context.Context) error {
	p.stops.Add(1)
	p.box.record("stop " + p.name)
	p.exit(ExecResult{RC: RCSignaled(15), Signal: "SIGTERM"})
	return nil
}

// exit — 让进程退出（幂等：第一个结果说了算）。模拟「半路死」就是不加 Stop 直接 exit(rc≠0)。
func (p *orchProc) exit(res ExecResult) {
	p.once.Do(func() {
		p.mu.Lock()
		p.res = res
		p.mu.Unlock()
		close(p.done)
	})
}

// orchBox — 事件流水 + 进程台账（Spawner/ProbeAddr 都记在这里，顺序可断言）。
type orchBox struct {
	mu     sync.Mutex
	events []string
	procs  map[string]*orchProc
	spawns map[string]int
	// addrs — 服务名 → 探测 "host:port"（缺省用 fallback）。
	addrs    map[string]string
	fallback string
}

func newOrchBox() *orchBox {
	return &orchBox{
		procs:  map[string]*orchProc{},
		spawns: map[string]int{},
		addrs:  map[string]string{},
	}
}

func (b *orchBox) record(s string) {
	b.mu.Lock()
	b.events = append(b.events, s)
	b.mu.Unlock()
}

func (b *orchBox) eventsSnapshot() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.events...)
}

// index — 某事件第一次出现的位置（没有 ⇒ -1）。
func (b *orchBox) index(ev string) int {
	for i, e := range b.eventsSnapshot() {
		if e == ev {
			return i
		}
	}
	return -1
}

func (b *orchBox) spawnCount(name string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.spawns[name]
}

func (b *orchBox) proc(name string) *orchProc {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.procs[name]
}

// die — 模拟「半路死」：进程自己退出，**不**经过 Stop。
func (b *orchBox) die(name string, res ExecResult) {
	b.record("die " + name)
	b.proc(name).exit(res)
}

// spawner — 可控 Spawner：记 spawn 事件、建长驻 Proc。
func (b *orchBox) spawner() Spawner {
	return SpawnFunc(func(ctx context.Context, be Backend, inst Instance, svc ServiceSpec, opts ExecOpts) (Proc, error) {
		b.mu.Lock()
		b.spawns[svc.Name]++
		if p, ok := b.procs[svc.Name]; ok {
			// 同一服务被起第二次（编排 bug）——用新 done 免得旧信号串场，但记下来。
			p = &orchProc{name: svc.Name, box: b, done: make(chan struct{})}
			b.procs[svc.Name] = p
		} else {
			b.procs[svc.Name] = &orchProc{name: svc.Name, box: b, done: make(chan struct{})}
		}
		p := b.procs[svc.Name]
		b.mu.Unlock()
		b.record("spawn " + svc.Name)
		return p, nil
	})
}

// probeAddr — 探测地址桩：记 probe 事件，按服务名给 host:port（缺省 fallback）。
func (b *orchBox) probeAddr() func(ServiceSpec, Readiness) (string, error) {
	return func(svc ServiceSpec, rd Readiness) (string, error) {
		b.record("probe " + svc.Name)
		b.mu.Lock()
		defer b.mu.Unlock()
		if a, ok := b.addrs[svc.Name]; ok {
			return a, nil
		}
		if b.fallback != "" {
			return b.fallback, nil
		}
		return "", fmt.Errorf("测试没给 %q 配探测地址", svc.Name)
	}
}

func (b *orchBox) optsFor() OrchestrateOptions {
	return OrchestrateOptions{
		Spawn:         b.spawner(),
		ProbeAddr:     b.probeAddr(),
		ProbeInterval: 2 * time.Millisecond,
		StopTimeout:   2 * time.Second,
	}
}

// hostPortOf — httptest 的 URL → "host:port"（探测桩只认 host:port）。
func hostPortOf(t *testing.T, rawURL string) string {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("解析 httptest URL %q: %v", rawURL, err)
	}
	return u.Host
}

// healthServer — /healthz 200，其余 503（用来验「只认 200」与「永不就绪」）。
func healthServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// waitFor — 轮询等一下异步事件（watcher/级联都是 goroutine 里发生的）。
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("等 %s 超时（3s）", what)
}

// ── ① 单服务起停 ──────────────────────────────────────────────────────

func TestOrchestrateSingleServiceStartStop(t *testing.T) {
	ctx, cancelStart := context.WithCancel(context.Background())
	defer cancelStart()
	be := NewFakeBackend()
	box := newOrchBox()

	sp := mustOrchSpec(t, `[[服务]]
name  = "web"
entry = ["webd", "--port", "8080"]
`)
	r, err := StartWith(ctx, be, sp, box.optsFor())
	if err != nil {
		t.Fatalf("单服务应当起得来，得到错误: %v", err)
	}
	if got, want := r.Started(), []string{"web"}; !slices.Equal(got, want) {
		t.Fatalf("Started = %v，want %v", got, want)
	}
	if box.spawnCount("web") != 1 {
		t.Fatalf("web 应当只被起一次，得到 %d 次", box.spawnCount("web"))
	}
	if inst := r.Instance(); inst.ID == "" || !be.Alive(inst) {
		t.Fatalf("实例应当活着，得到 %v（LiveIDs=%v）", inst, be.LiveIDs())
	}

	// ★ Start 的 ctx 只管启动阶段：调用方 defer cancel() 不该顺手杀掉已经起来的服务。
	cancelStart()
	time.Sleep(5 * time.Millisecond)
	if st0 := r.States()[0]; st0.Stopped || st0.Exited {
		t.Fatalf("Start 的 ctx 取消不该停服务，得到 %+v", st0)
	}
	if n := box.proc("web").stops.Load(); n != 0 {
		t.Fatalf("Start 的 ctx 取消不该停进程，得到 %d 次 Stop", n)
	}

	// 未声明 readiness ⇒ started 即就绪，日志要说明白为什么这么判。
	logs := r.Log()
	if !slices.ContainsFunc(logs, func(s string) bool {
		return strings.Contains(s, "ready web") && strings.Contains(s, "未声明 readiness")
	}) {
		t.Fatalf("日志里应有一句「未声明 readiness ⇒ started 即就绪」，得到 %v", logs)
	}

	if err := Stop(ctx, r); err != nil {
		t.Fatalf("Stop 应当成功，得到 %v", err)
	}
	if n := box.proc("web").stops.Load(); n != 1 {
		t.Fatalf("web 应当被停 1 次，得到 %d 次", n)
	}
	st := r.States()
	if len(st) != 1 || !st[0].Stopped || st[0].Ready != true {
		t.Fatalf("States 应显示 web 已就绪且已停，得到 %+v", st)
	}
	if stops := r.Stops(); len(stops) != 1 || stops[0].Service != "web" || stops[0].Because != "Stop" {
		t.Fatalf("Stops 应记一条 web/Stop，得到 %+v", stops)
	}
	// Stop 不 Destroy：销毁是调用方的事（这里实例应当还活着）。
	if !be.Alive(r.Instance()) {
		t.Fatal("Stop 不该顺手 Destroy 实例（销毁归调用方）")
	}
}

// ── ② 顺序边：api 必须等 db 就绪 ──────────────────────────────────────

func TestOrchestrateOrderEdgeBlocksOnDep(t *testing.T) {
	ctx := context.Background()

	t.Run("db 就绪之后才起 api", func(t *testing.T) {
		be := NewFakeBackend()
		box := newOrchBox()
		srv := healthServer(t)
		box.fallback = hostPortOf(t, srv.URL)

		// 声明顺序故意的反着写（api 在 db 前）：排序只认 depends_on，不认文件里的先后。
		sp := mustOrchSpec(t, svcAPIDepDB+svcDB)
		if got, want := sp.Order, []string{"db", "api"}; !slices.Equal(got, want) {
			t.Fatalf("解析器给的拓扑序 = %v，want %v", got, want)
		}

		r, err := StartWith(ctx, be, sp, box.optsFor())
		if err != nil {
			t.Fatalf("db→api 应当起得来，得到错误: %v", err)
		}
		t.Cleanup(func() { _ = Stop(ctx, r) })

		if got, want := r.Started(), []string{"db", "api"}; !slices.Equal(got, want) {
			t.Fatalf("Started = %v，want %v（顺序边要求依赖在前）", got, want)
		}
		iSpawnDB := box.index("spawn db")
		iProbeDB := box.index("probe db")
		iSpawnAPI := box.index("spawn api")
		if iSpawnDB < 0 || iProbeDB < 0 || iSpawnAPI < 0 {
			t.Fatalf("事件不全：%v", box.eventsSnapshot())
		}
		if !(iSpawnDB < iProbeDB && iProbeDB < iSpawnAPI) {
			t.Fatalf("顺序边没生效：要求 spawn db < probe db < spawn api，得到 %v", box.eventsSnapshot())
		}
		if r.States()[0].Ready != true || r.States()[1].Ready != true {
			t.Fatalf("两个服务都应就绪，得到 %+v", r.States())
		}
	})

	t.Run("db 永不就绪 ⇒ api 一次都不许起", func(t *testing.T) {
		be := NewFakeBackend()
		box := newOrchBox()
		srv := healthServer(t)
		box.fallback = hostPortOf(t, srv.URL)

		// db 的 readiness 指到 /down（503），timeout 给短一点。
		sp := mustOrchSpec(t, `[[服务]]
name      = "db"
entry     = ["postgres"]
readiness = { port = 5432, path = "/down", timeout = "150ms" }
`+svcAPIDepDB)

		_, err := StartWith(ctx, be, sp, box.optsFor())
		if err == nil {
			t.Fatal("db 永不就绪时 Start 必须失败")
		}
		if !errors.Is(err, ErrReadinessTimeout) {
			t.Fatalf("错误应是 ErrReadinessTimeout，得到 %v", err)
		}
		var oe *OrchestrateError
		if !errors.As(err, &oe) || oe.Service != "db" {
			t.Fatalf("错误应点名 db，得到 %v", err)
		}
		if n := box.spawnCount("api"); n != 0 {
			t.Fatalf("db 没就绪时不许起 api，实际起了 %d 次（事件：%v）", n, box.eventsSnapshot())
		}
		// 回滚把 db 自己停掉（它也进了「本次已起」），实例被 Destroy。
		if n := box.proc("db").stops.Load(); n != 1 {
			t.Fatalf("回滚应停掉 db 1 次，得到 %d 次", n)
		}
		if live := be.LiveIDs(); len(live) != 0 {
			t.Fatalf("回滚后不该留实例，得到 %v", live)
		}
	})
}

// ── ③ 存活边：db 半路死 ⇒ api 被级联停（★ 最关键那格）────────────────

func TestOrchestrateCascadeStopOnDependencyDeath(t *testing.T) {
	ctx := context.Background()
	be := NewFakeBackend()
	box := newOrchBox()
	srv := healthServer(t)
	box.fallback = hostPortOf(t, srv.URL)

	sp := mustOrchSpec(t, svcDB+svcAPIDepDB)
	r, err := StartWith(ctx, be, sp, box.optsFor())
	if err != nil {
		t.Fatalf("db→api 应当起得来，得到错误: %v", err)
	}
	if got, want := r.Started(), []string{"db", "api"}; !slices.Equal(got, want) {
		t.Fatalf("Started = %v，want %v", got, want)
	}

	// ★ db 半路死：进程自己以 rc=1 退出（不是被 Stop）。
	box.die("db", ExecResult{RC: 1})

	waitFor(t, "api 被级联停", func() bool {
		if n := box.proc("api").stops.Load(); n > 0 {
			return true
		}
		return false
	})

	if n := box.proc("api").stops.Load(); n != 1 {
		t.Fatalf("api 应被停 1 次，得到 %d 次", n)
	}
	if n := box.proc("db").stops.Load(); n != 0 {
		t.Fatalf("db 是自己死的，不该被 Stop（得到 %d 次）", n)
	}

	st := r.States()
	if len(st) != 2 {
		t.Fatalf("States 应有 2 个，得到 %+v", st)
	}
	if !st[0].Dead || st[0].Exit.RC != 1 {
		t.Fatalf("db 应判 Dead 且 rc=1，得到 %+v", st[0])
	}
	if !st[1].Stopped || st[1].StopReason == "" {
		t.Fatalf("api 应判 Stopped 且带原因，得到 %+v", st[1])
	}
	if st[1].Ready != true {
		t.Fatalf("api 停之前是就绪的，States 不该抹掉这件事：%+v", st[1])
	}

	stops := r.Stops()
	if len(stops) != 1 {
		got := ""
		for _, s := range r.Log() {
			got += "\n  " + s
		}
		t.Fatalf("Stops 应恰有一条（api），得到 %+v；日志：%s", stops, got)
	}
	if stops[0].Service != "api" || stops[0].Because != "db" {
		t.Fatalf("级联停的肇因应是 db，得到 %+v", stops[0])
	}
	if want := "因为 db 已退出（rc=1）"; stops[0].Reason != want {
		t.Fatalf("Reason = %q，want %q", stops[0].Reason, want)
	}

	logs := strings.Join(r.Log(), "\n")
	if !strings.Contains(logs, "died db: rc=1") {
		t.Fatalf("日志要记 db 怎么没的，得到：\n%s", logs)
	}
	if !strings.Contains(logs, "stop api: 因为 db 已退出（rc=1）") {
		t.Fatalf("日志要逐条写「因为谁停的」，得到：\n%s", logs)
	}

	// 停了之后再来一次 Stop：不该再停一遍（api 已停、db 已退）。
	before := len(r.Stops())
	if err := Stop(ctx, r); err != nil {
		t.Fatalf("级联停之后 Stop 应当仍成功，得到 %v", err)
	}
	if after := len(r.Stops()); after != before {
		t.Fatalf("Stop 不该重复记停（%d → %d）", before, after)
	}

	t.Run("传递级联：web 因为 api、api 因为 db", func(t *testing.T) {
		be := NewFakeBackend()
		box := newOrchBox()
		box.fallback = hostPortOf(t, healthServer(t).URL)

		sp := mustOrchSpec(t, svcDB+svcAPIDepDB+svcWebDepAPI)
		r, err := StartWith(ctx, be, sp, box.optsFor())
		if err != nil {
			t.Fatalf("三服务链应当起得来，得到错误: %v", err)
		}
		box.die("db", ExecResult{RC: 137, Signal: "SIGKILL"})

		waitFor(t, "web 被传递级联停", func() bool {
			return box.proc("web").stops.Load() > 0
		})
		stops := r.Stops()
		if len(stops) != 2 {
			t.Fatalf("应有 2 条停（web、api），得到 %+v；日志：\n%s", stops, strings.Join(r.Log(), "\n"))
		}
		// 逆序：先停最下游 web，再停 api；且「因为谁」取最近的那一环，不是一律记 db。
		if stops[0].Service != "web" || stops[0].Because != "api" {
			t.Fatalf("第一条应是 web 因为 api，得到 %+v", stops[0])
		}
		if stops[1].Service != "api" || stops[1].Because != "db" {
			t.Fatalf("第二条应是 api 因为 db，得到 %+v", stops[1])
		}
		logs := strings.Join(r.Log(), "\n")
		if !strings.Contains(logs, "stop web: 因为 api 已随 db 停掉（db 已退出，被 SIGKILL 杀死（RC=137））") {
			t.Fatalf("传递级联的日志要把链条写清，得到：\n%s", logs)
		}
	})
}

// ── ④ 就绪超时 ⇒ 逆序回滚 ────────────────────────────────────────────

func TestOrchestrateReadinessTimeoutRollsBack(t *testing.T) {
	ctx := context.Background()
	be := NewFakeBackend()
	box := newOrchBox()
	srv := healthServer(t)
	box.fallback = hostPortOf(t, srv.URL)

	// db 就绪（/healthz 200），api 永不就绪（/down 503）⇒ 已起集合是 [db, api]，
	// 回滚必须按**启动逆序**停成 [api, db]。
	sp := mustOrchSpec(t, svcDB+`[[服务]]
name       = "api"
entry      = ["uvicorn", "app:api"]
depends_on = ["db"]
readiness  = { port = 8000, path = "/down", timeout = "120ms" }
`)
	_, err := StartWith(ctx, be, sp, box.optsFor())
	if err == nil {
		t.Fatal("api 永不就绪时 Start 必须失败（四家都不给多服务事务，我们自己补回滚）")
	}
	if !errors.Is(err, ErrReadinessTimeout) {
		t.Fatalf("错误应是 ErrReadinessTimeout，得到 %v", err)
	}
	var oe *OrchestrateError
	if !errors.As(err, &oe) {
		t.Fatalf("错误应是 *OrchestrateError，得到 %T", err)
	}
	if oe.Service != "api" {
		t.Fatalf("错误要点名 api，得到 %q", oe.Service)
	}
	if want := []string{"api", "db"}; !slices.Equal(oe.Stopped, want) {
		t.Fatalf("回滚顺序应是启动逆序 %v，得到 %v", want, oe.Stopped)
	}
	msg := err.Error()
	for _, want := range []string{`服务 "api"`, "只认 200", "已按启动逆序停掉 2 个：api, db", "120ms"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("错误文案缺 %q：%s", want, msg)
		}
	}
	// 两个都被停，且顺序是 api 先、db 后。
	if n := box.proc("api").stops.Load(); n != 1 {
		t.Fatalf("api 应被停 1 次，得到 %d", n)
	}
	if n := box.proc("db").stops.Load(); n != 1 {
		t.Fatalf("db 应被停 1 次，得到 %d", n)
	}
	iAPI, iDB := box.index("stop api"), box.index("stop db")
	if iAPI < 0 || iDB < 0 || iAPI > iDB {
		t.Fatalf("回滚必须逆序（api 先停）：%v", box.eventsSnapshot())
	}
	// 失败路径上实例是 Start 自己 Provision 的 ⇒ 必须 Destroy，不能漏账。
	if live := be.LiveIDs(); len(live) != 0 {
		t.Fatalf("回滚后不该留实例，得到 %v", live)
	}
	if !slices.ContainsFunc(be.Calls(), func(s string) bool { return strings.HasPrefix(s, "destroy ") }) {
		t.Fatalf("回滚必须 Destroy 实例，调用流水：%v", be.Calls())
	}
}

// ── ⑤ 防御性拒环 ──────────────────────────────────────────────────────

func TestOrchestrateRejectsBadGraphDefensively(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name    string
		spec    *Spec
		wantErr error
		wantMsg string
	}{
		{
			name: "环依赖（手搓 Spec 绕过解析器）",
			spec: &Spec{Services: []ServiceSpec{
				{Name: "a", Entry: []string{"a"}, DependsOn: []string{"b"}},
				{Name: "b", Entry: []string{"b"}, DependsOn: []string{"a"}},
			}},
			wantErr: ErrCycle,
			wantMsg: "a → b → a",
		},
		{
			name: "自依赖",
			spec: &Spec{Services: []ServiceSpec{
				{Name: "a", Entry: []string{"a"}, DependsOn: []string{"a"}},
			}},
			wantErr: ErrCycle,
			wantMsg: "依赖自身",
		},
		{
			name: "悬空依赖",
			spec: &Spec{Services: []ServiceSpec{
				{Name: "a", Entry: []string{"a"}, DependsOn: []string{"ghost"}},
			}},
			wantErr: ErrDanglingDep,
			wantMsg: `"ghost"`,
		},
		{
			name: "Order 违反拓扑序",
			spec: &Spec{
				Services: []ServiceSpec{
					{Name: "db", Entry: []string{"db"}},
					{Name: "api", Entry: []string{"api"}, DependsOn: []string{"db"}},
				},
				Order: []string{"api", "db"},
			},
			wantErr: ErrBadOrder,
			wantMsg: "违反拓扑序",
		},
		{
			name: "Order 漏了服务",
			spec: &Spec{
				Services: []ServiceSpec{
					{Name: "db", Entry: []string{"db"}},
					{Name: "api", Entry: []string{"api"}},
				},
				Order: []string{"db"},
			},
			wantErr: ErrBadOrder,
			wantMsg: "永远不起",
		},
		{
			name: "服务重名",
			spec: &Spec{Services: []ServiceSpec{
				{Name: "a", Entry: []string{"a"}},
				{Name: "a", Entry: []string{"a2"}},
			}},
			wantErr: ErrBadOrder,
			wantMsg: "重名",
		},
		{
			name:    "一个服务都没有",
			spec:    &Spec{},
			wantErr: ErrNoServices,
			wantMsg: "至少需要一个",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			be := NewFakeBackend()
			_, err := StartWith(ctx, be, tc.spec, OrchestrateOptions{StopTimeout: time.Second})
			if err == nil {
				t.Fatal("非法依赖图必须被拒")
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("errors.Is(%v, %v) 应为真，得到错误 %v", err, tc.wantErr, err)
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Fatalf("错误文案缺 %q：%s", tc.wantMsg, err.Error())
			}
			// ★ 拒在副作用之前：一个 Provision 都不许发生。
			if calls := be.Calls(); len(calls) != 0 {
				t.Fatalf("非法图必须在 Provision 之前被拒，实际流水：%v", calls)
			}
		})
	}

	// nil 契约 / nil 后端也得报错，不许 panic。
	if _, err := StartWith(ctx, NewFakeBackend(), nil, OrchestrateOptions{}); err == nil {
		t.Fatal("nil 契约必须报错")
	}
	if _, err := StartWith(ctx, nil, &Spec{}, OrchestrateOptions{}); err == nil {
		t.Fatal("nil 后端必须报错")
	}
}

// ── ⑥ Stop 幂等 ───────────────────────────────────────────────────────

func TestOrchestrateStopIdempotent(t *testing.T) {
	ctx := context.Background()
	be := NewFakeBackend()
	box := newOrchBox()
	box.fallback = hostPortOf(t, healthServer(t).URL)

	sp := mustOrchSpec(t, svcDB+svcAPIDepDB+svcWebDepAPI)
	r, err := StartWith(ctx, be, sp, box.optsFor())
	if err != nil {
		t.Fatalf("三服务链应当起得来，得到错误: %v", err)
	}
	if got, want := r.Started(), []string{"db", "api", "web"}; !slices.Equal(got, want) {
		t.Fatalf("Started = %v，want %v", got, want)
	}

	if err := r.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	firstStops := len(r.Stops())
	firstLogs := len(r.Log())
	if firstStops != 3 {
		t.Fatalf("三个服务都应被停，Stops = %+v", r.Stops())
	}

	// 第二次 Stop：同一结果、不再停第二遍、不再写日志。
	if err := Stop(ctx, r); err != nil {
		t.Fatalf("第二次 Stop 应当返回 nil（幂等），得到 %v", err)
	}
	for _, name := range []string{"db", "api", "web"} {
		if n := box.proc(name).stops.Load(); n != 1 {
			t.Fatalf("%s 应恰被停 1 次，得到 %d 次", name, n)
		}
	}
	if got := len(r.Stops()); got != firstStops {
		t.Fatalf("幂等：Stops 条数不该变（%d → %d）", firstStops, got)
	}
	if got := len(r.Log()); got != firstLogs {
		t.Fatalf("幂等：重复 Stop 不该再写日志（%d → %d）:\n%s", firstLogs, got, strings.Join(r.Log(), "\n"))
	}

	// 零值 Running：Stop 是 no-op（defer 里好用），不是 error。
	var zero Running
	if zero.Valid() {
		t.Fatal("零值 Running 不该是 Valid")
	}
	if err := zero.Stop(ctx); err != nil {
		t.Fatalf("零值 Running 的 Stop 应当是 nil，得到 %v", err)
	}

	// 并发两次 Stop：只能停一遍，两边都拿到 nil。
	be2 := NewFakeBackend()
	box2 := newOrchBox()
	box2.fallback = hostPortOf(t, healthServer(t).URL)
	r2, err := StartWith(ctx, be2, sp, box2.optsFor())
	if err != nil {
		t.Fatalf("第二次编排应当起得来，得到错误: %v", err)
	}
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = Stop(ctx, r2)
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("并发 Stop[%d] 应为 nil，得到 %v", i, err)
		}
	}
	for _, name := range []string{"db", "api", "web"} {
		if n := box2.proc(name).stops.Load(); n != 1 {
			t.Fatalf("并发 Stop 下 %s 应恰被停 1 次，得到 %d 次", name, n)
		}
	}
	if got := len(r2.Stops()); got != 3 {
		t.Fatalf("并发 Stop 后 Stops 应为 3 条，得到 %d：%+v", got, r2.Stops())
	}
}

// ── ⑦ 停的日志：逆序 + 「因为谁」──────────────────────────────────────

func TestOrchestrateStopLogsBecauseWhom(t *testing.T) {
	ctx := context.Background()
	be := NewFakeBackend()
	box := newOrchBox()
	box.fallback = hostPortOf(t, healthServer(t).URL)

	sp := mustOrchSpec(t, svcDB+svcAPIDepDB+svcWebDepAPI)
	r, err := StartWith(ctx, be, sp, box.optsFor())
	if err != nil {
		t.Fatalf("三服务链应当起得来，得到错误: %v", err)
	}
	if err := Stop(ctx, r); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	logs := r.Log()
	// 逆序：web → api → db（先停下游，后停上游）。
	iWeb, iAPI, iDB := -1, -1, -1
	for i, line := range logs {
		switch {
		case strings.HasPrefix(line, "stop web:"):
			iWeb = i
		case strings.HasPrefix(line, "stop api:"):
			iAPI = i
		case strings.HasPrefix(line, "stop db:"):
			iDB = i
		}
	}
	if iWeb < 0 || iAPI < 0 || iDB < 0 {
		t.Fatalf("三条停日志都要在，得到：\n%s", strings.Join(logs, "\n"))
	}
	if !(iWeb < iAPI && iAPI < iDB) {
		t.Fatalf("停必须逆序（web < api < db），得到：\n%s", strings.Join(logs, "\n"))
	}
	for _, line := range logs {
		if strings.HasPrefix(line, "stop ") && !strings.Contains(line, "因为") {
			t.Fatalf("每条停日志都要写「因为谁」：%q", line)
		}
	}
	wantLine := "stop web: 因为 Stop 请求（按启动逆序）"
	if !slices.Contains(logs, wantLine) {
		t.Fatalf("日志里应有一行 %q，得到：\n%s", wantLine, strings.Join(logs, "\n"))
	}
	for _, ev := range r.Stops() {
		if ev.Because != "Stop" || !strings.HasPrefix(ev.Reason, "因为 Stop 请求") {
			t.Fatalf("StopEvent 应记 because=Stop，得到 %+v", ev)
		}
	}
}

// ── ⑧ 缺省 Spawner：真走 Backend.Exec ─────────────────────────────────

func TestOrchestrateDefaultSpawnerUsesBackend(t *testing.T) {
	ctx := context.Background()
	be := NewFakeBackend()
	sp := mustOrchSpec(t, `[[服务]]
name  = "web"
entry = ["webd", "--port", "8080"]
`)
	// 缺省 Spawner：Exec 的返回值就是进程的退出观测（脚本化 rc=0 ⇒ compose completed）。
	r, err := StartWith(ctx, be, sp, OrchestrateOptions{StopTimeout: time.Second})
	if err != nil {
		t.Fatalf("单服务应当起得来，得到错误: %v", err)
	}
	t.Cleanup(func() { _ = Stop(ctx, r) })

	waitFor(t, "web 的 Exec 跑完", func() bool {
		st := r.States()
		return len(st) == 1 && st[0].Exited
	})
	st := r.States()[0]
	if st.Exit.RC != 0 || st.Dead {
		t.Fatalf("rc=0 的退出应判 completed（Dead=false），得到 %+v", st)
	}

	last, ok := be.LastExec(r.Instance())
	if !ok {
		t.Fatal("缺省 Spawner 必须真的调 Backend.Exec")
	}
	if want := []string{"webd", "--port", "8080"}; !slices.Equal(last.Argv, want) {
		t.Fatalf("argv 透传错：%v，want %v", last.Argv, want)
	}
	if last.Opts.Dir != "/app" {
		t.Fatalf("Dir 应取 茧.workdir=/app，得到 %q", last.Opts.Dir)
	}
	if last.Opts.Env["TZ"] != "Asia/Shanghai" || last.Opts.Env["LANG"] != "C.UTF-8" {
		t.Fatalf("Env 应带契约默认值，得到 %v", last.Opts.Env)
	}
	if last.Opts.Timeout != 0 {
		t.Fatalf("服务的 Exec 不该设 Timeout（那是给长驻进程判死期），得到 %v", last.Opts.Timeout)
	}
	// completed 不触发存活边：日志里应是「正常完成」，不是「died」。
	logs := strings.Join(r.Log(), "\n")
	if !strings.Contains(logs, "exit web: 正常完成") || strings.Contains(logs, "died web") {
		t.Fatalf("rc=0 应记「正常完成」且不判死，得到：\n%s", logs)
	}

	// 已经自己退完的服务：Stop 是 no-op，但仍要有话说（不然「停成功」和「没东西可停」分不开）。
	if err := Stop(ctx, r); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if n := len(r.Stops()); n != 0 {
		t.Fatalf("已完成的服务不该进 Stops，得到 %+v", r.Stops())
	}
	if !strings.Contains(strings.Join(r.Log(), "\n"), "无存活服务（幂等 no-op）") {
		t.Fatalf("没有可停的东西时日志要说清楚，得到：\n%s", strings.Join(r.Log(), "\n"))
	}
}

// ── ⑨ readiness 无 path ⇒ 端口可连即就绪 ─────────────────────────────

func TestOrchestrateEmptyPathProbesTCP(t *testing.T) {
	ctx := context.Background()
	be := NewFakeBackend()
	box := newOrchBox()
	srv := healthServer(t)
	box.fallback = hostPortOf(t, srv.URL)

	// path 为空：只要求端口在听（v2 §D 的 readiness 允许只写 port）。
	sp := mustOrchSpec(t, `[[服务]]
name      = "db"
entry     = ["postgres"]
readiness = { port = 5432, timeout = "500ms" }
`)
	r, err := StartWith(ctx, be, sp, box.optsFor())
	if err != nil {
		t.Fatalf("端口可连时应当就绪，得到错误: %v", err)
	}
	t.Cleanup(func() { _ = Stop(ctx, r) })
	if st := r.States(); len(st) != 1 || !st[0].Ready {
		t.Fatalf("db 应就绪，得到 %+v", st)
	}
	if !strings.Contains(strings.Join(r.Log(), "\n"), "tcp://") {
		t.Fatalf("无 path 时应探 TCP 而不是 HTTP，得到：\n%s", strings.Join(r.Log(), "\n"))
	}
}

// ── ⑩ 后台档：Exec 返回 ≠ 进程退出（T3c 的编排层判据）──────────────────────

// orchDetachBackend — 演真后台档后端：签出 pid 即返回，退出观测交给 DetachedProbe。
type orchDetachBackend struct {
	*FakeBackend
	mu     sync.Mutex
	pid    int
	exited bool
	exitRC int
	polls  int
	detach bool
}

func (b *orchDetachBackend) Exec(ctx context.Context, inst Instance, argv []string, opts ExecOpts) (ExecResult, error) {
	b.mu.Lock()
	b.detach = opts.Detach
	b.mu.Unlock()
	if !opts.Detach {
		return b.FakeBackend.Exec(ctx, inst, argv, opts)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return ExecResult{RC: 0, PID: b.pid}, nil
}

func (b *orchDetachBackend) DetachedStatus(pid int) (bool, bool, ExecResult) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.polls++
	if b.exited {
		return false, true, ExecResult{RC: b.exitRC}
	}
	return true, false, ExecResult{}
}

func (b *orchDetachBackend) kill(rc int) {
	b.mu.Lock()
	b.exited, b.exitRC = true, rc
	b.mu.Unlock()
}

// TestOrchestrateDetachDoesNotCountSignOutAsExit — 后台档 Exec 立刻返回（签出即返回），
// 编排层**不许**把它当「进程已退」：
//
//	① 该服务在就绪前不该被判死（否则每个后台档服务都永远起不来）；
//	② 探针说「退了」之后，存活边才生效（rc≠0 ⇒ 判死）。
func TestOrchestrateDetachDoesNotCountSignOutAsExit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	be := &orchDetachBackend{FakeBackend: NewFakeBackend(), pid: 4242, exitRC: 3}
	srv := healthServer(t)
	box := newOrchBox()
	box.fallback = hostPortOf(t, srv.URL)

	sp := mustOrchSpec(t, `[[服务]]
name      = "api"
entry     = ["uvicorn", "app:api"]
readiness = { port = 8000, path = "/healthz", timeout = "2s" }
`)
	opts := box.optsFor()
	opts.Detach = true
	// 后台档必须落文件（ExecOpts.Validate 会拒「Detach 而无 LogPath」）⇒ 给一格临时日志目录。
	opts.LogDir = t.TempDir()
	// 用缺省 Spawner（本格要验的正是它），只留探测桩与短超时。
	opts.Spawn = nil
	r, err := StartWith(ctx, be, sp, opts)
	if err != nil {
		t.Fatalf("后台档签出后应当起得来（Exec 返回 ≠ 退出），得到错误: %v", err)
	}
	t.Cleanup(func() { _ = Stop(ctx, r) })

	if st := r.States(); len(st) != 1 || !st[0].Ready || st[0].Exited {
		t.Fatalf("签出后该是「就绪且未退」，得到 %+v", st)
	}
	be.mu.Lock()
	polls, detach := be.polls, be.detach
	be.mu.Unlock()
	if !detach {
		t.Fatalf("Detach 没传到后端 Exec（服务又会被起它的进程带走）")
	}
	if polls == 0 {
		t.Fatalf("一次判活探针都没打")
	}

	// 探针改口：进程真退了（rc=3）⇒ 存活边判死。
	be.kill(3)
	waitFor(t, "api 判死", func() bool {
		st := r.States()
		return len(st) == 1 && st[0].Exited
	})
	st := r.States()[0]
	if !st.Dead || st.Exit.RC != 3 {
		t.Fatalf("rc=3 的退出该判死，得到 %+v", st)
	}
}

// ── ⑪ AI 注入面：Env 合并 ＋ 挂载面（T4）────────────────────────────────

// TestOrchestrateAIEnvAndMounts — 编排层替命令面做两件事，两件都要有牙：
//
//	① 把 AI 注入面（三个 env）并进**每个**服务的 Exec，且**同名以注入面为准** ——
//	   否则契约 [环境] 里一句 ZERG_AI_BASE 就能把茧指向任意 URL（"base 由宿主给"就成了装饰）。
//	② 在 Provision 之后、**第一个服务起之前**逐条挂好（nsjail 的 --bindmount 只能起进程时给）；
//	   挂不上 ⇒ 整次 Start 失败且实例已销毁（✗ 不许"少挂一条照跑"）。
func TestOrchestrateAIEnvAndMounts(t *testing.T) {
	ctx := context.Background()
	oneSvc := `[[服务]]
name  = "web"
entry = ["webd"]
`

	t.Run("三个 env 并进去且同名以注入面为准", func(t *testing.T) {
		be := NewFakeBackend()
		sp := mustOrchSpec(t, oneSvc)
		// 契约里的同名键必须被盖掉；不同名的键必须留着（注入面不是"清空 env"）。
		sp.Env[EnvAIBase] = "http://<cluster-ip>:9999"
		sp.Env["KEEP"] = "kept"

		aiEnv := map[string]string{
			EnvAIBase:      "http://127.0.0.1:8082",
			EnvAIModel:     "m-1",
			EnvAITokenFile: AITokenInJailPath,
		}
		opts := OrchestrateOptions{
			StopTimeout: time.Second,
			AIEnv:       aiEnv,
			Mounts:      []OrchestrateMount{{Host: "/host/shop-1/ai.token", Inner: AITokenInJailPath, RO: true}},
		}
		r, err := StartWith(ctx, be, sp, opts)
		if err != nil {
			t.Fatalf("应当起得来，得到错误: %v", err)
		}
		t.Cleanup(func() { _ = Stop(ctx, r) })

		// ★ 缺省 Spawner 在 **goroutine** 里调 Exec ⇒ 先等它真跑完（rc=0 ⇒ completed），
		//   否则这条断言会在"还没 exec"的瞬间读到空（那不是缺陷，是竞态）。
		waitFor(t, "web 的 Exec 跑完", func() bool {
			st := r.States()
			return len(st) == 1 && st[0].Exited
		})
		last, ok := be.LastExec(r.Instance())
		if !ok {
			t.Fatal("缺省 Spawner 必须真的调 Backend.Exec")
		}
		for k, want := range aiEnv {
			if got := last.Opts.Env[k]; got != want {
				t.Fatalf("Exec 的 env[%s] = %q，want %q（注入面没并进服务 env）", k, got, want)
			}
		}
		if last.Opts.Env["KEEP"] != "kept" {
			t.Fatalf("契约自己的 env 键被丢了：%v", last.Opts.Env)
		}

		mounts := be.Mounts(r.Instance())
		if len(mounts) != 1 {
			t.Fatalf("应当恰好一条挂载，得到 %+v", mounts)
		}
		if mounts[0].Inner != AITokenInJailPath || mounts[0].Host != "/host/shop-1/ai.token" || !mounts[0].RO {
			t.Fatalf("挂载项不符（令牌件必须是 ro 绑定）：%+v", mounts[0])
		}

		// 顺序判据：mount 必须排在 exec 之前（挂载只能"起进程时给"）。
		calls := be.Calls()
		iMount, iExec := -1, -1
		for i, c := range calls {
			if strings.HasPrefix(c, "mount ") && iMount < 0 {
				iMount = i
			}
			if strings.HasPrefix(c, "exec ") && iExec < 0 {
				iExec = i
			}
		}
		if iMount < 0 || iExec < 0 || iMount > iExec {
			t.Fatalf("mount 必须早于 exec（mount=%d exec=%d）：%v", iMount, iExec, calls)
		}
	})

	t.Run("挂不上 ⇒ 整次失败且实例已销毁", func(t *testing.T) {
		be := NewFakeBackend()
		sp := mustOrchSpec(t, oneSvc)
		// 相对路径：后端拒（ErrBadMount），编排层要把它变成"整次失败"而不是"少挂一条照跑"。
		_, err := StartWith(ctx, be, sp, OrchestrateOptions{
			StopTimeout: time.Second,
			Mounts:      []OrchestrateMount{{Host: "rel/ai.token", Inner: AITokenInJailPath, RO: true}},
		})
		if err == nil {
			t.Fatal("挂载失败时 Start 必须报错（宁红不假绿）")
		}
		for _, c := range be.Calls() {
			if strings.HasPrefix(c, "exec ") {
				t.Fatalf("挂载失败后不该起任何服务，却见到：%s", c)
			}
		}
		if ids := be.LiveIDs(); len(ids) != 0 {
			t.Fatalf("挂载失败后实例必须被 Destroy，仍活着：%v", ids)
		}
	})
}
