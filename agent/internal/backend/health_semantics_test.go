package backend

// health_semantics_test.go —— 健康口径（Mr2109 2026-09-15 拍板「空着也算健康」）的回归判据。
//
// 背景：卵的设计是「默认空着」（§1.1）。旧口径「至少一个 ready 驻留且 /health 通过」
// ⇒ 空着恒 false ⇒ 主控 healthy_count 恒 0；而 master_scheduler 的「等待任务重派」
// 双条件（熔断冷却过 + 机器 healthy）会因此永远不满足 ⇒ 空着的机器接不到重派。
//
// 判据：
//  ① 无驻留（空着）⇒ IsHealthy == true；
//  ② 只有 crashed 驻留 ⇒ false（真不健康，信号保留）；
//  ③ ready 但自检不过（端口没人听）⇒ false。
import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"
)

func TestHealthSemantics_EmptyIsHealthy(t *testing.T) {
	m := newEvictTestManager(1, map[string]*subproc{})
	if !m.IsHealthy() {
		t.Fatal("空着（无驻留）必须算健康——否则主控看板恒 unhealthy、等待任务永不重派")
	}
}

func TestHealthSemantics_AllCrashedIsUnhealthy(t *testing.T) {
	m := newEvictTestManager(1, map[string]*subproc{
		"dead": {model: "dead", state: StateCrashed, lastUsed: time.Now(), entry: inflightEntry()},
	})
	if m.IsHealthy() {
		t.Fatal("全部驻留崩溃 ⇒ 必须 unhealthy")
	}
}

func TestHealthSemantics_ReadyButDeadPortIsUnhealthy(t *testing.T) {
	m := newEvictTestManager(1, map[string]*subproc{
		// 9411 上没有任何监听 ⇒ healthCheck 必失败
		"ghost": {model: "ghost", state: StateReady, port: 9411, lastUsed: time.Now(), entry: inflightEntry()},
	})
	if m.IsHealthy() {
		t.Fatal("ready 但自检不过 ⇒ 必须 unhealthy")
	}
}

// 回归：空窗计时中（idle_armed）的卵是活的 ⇒ 必须算健康。
// 2026-09-15 生产实测抓到旧实现漏了它：卵好好地在空窗里，/status.healthy 却是 false。
func TestHealthSemantics_IdleArmedCountsHealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK) // /health、/v1/models 都 200
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	p, _ := strconv.Atoi(u.Port())

	m := newEvictTestManager(1, map[string]*subproc{
		"idle": {model: "idle", state: StateIdleArmed, port: p, lastUsed: time.Now(), entry: inflightEntry()},
	})
	if !m.IsHealthy() {
		t.Fatal("空窗计时中的卵是活的、能服务 ⇒ 必须算健康（否则空窗窗口内主控又看到 unhealthy）")
	}
}
