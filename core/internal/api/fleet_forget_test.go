package api

// fleet_forget_test.go —— `POST /api/fleet/forget`（GAP-20261010-21 · A 单·端点面）的回归牙。
//
// 覆盖三面：① handler 语义（干跑只报计划 · 真删落 store · confirm 闸 · 幂等 · 坏体/空名 400）；
// ② 路由真源面 —— `main.go` 的**真实注册行**决定 chi 认得它（200）还是未知路由（404）
// （★ 结论由扫真源驱动，✗ 不硬编码；改前跑必得 404、改后跑必得 200）。

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/store"
	"github.com/go-chi/chi/v5"
)

func newForgetHandlers() *Handlers {
	return &Handlers{
		Store:           store.NewStore(),
		HeartbeatLogger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func seedForgetMachine(h *Handlers, name string) {
	h.Store.ReceiveHeartbeat(store.HeartbeatRequest{Machine: name})
}

func doFleetForget(t *testing.T, h *Handlers, body string) (int, map[string]interface{}) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/fleet/forget", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.FleetForgetHandler(w, req)
	out := map[string]interface{}{}
	if w.Body.Len() > 0 {
		_ = json.Unmarshal(w.Body.Bytes(), &out)
	}
	return w.Code, out
}

// TestFleetForgetHandler_DryRunReportsPlan —— 干跑只报计划，一个字节都不动名册。
func TestFleetForgetHandler_DryRunReportsPlan(t *testing.T) {
	h := newForgetHandlers()
	seedForgetMachine(h, "probe")
	code, out := doFleetForget(t, h, `{"machine":"probe","dry_run":true}`)
	if code != http.StatusOK {
		t.Fatalf("干跑应 200，实得 %d（%v）", code, out)
	}
	if out["dry_run"] != true || out["would_forget"] != true {
		t.Fatalf("干跑应报 dry_run=true 且 would_forget=true：%v", out)
	}
	if h.Store.GetSnapshot("probe") == nil {
		t.Fatal("干跑不得动名册（probe 应仍在册）")
	}
}

// TestFleetForgetHandler_ConfirmRequired —— 缺 confirm 的真删请求 fail-closed 400，名册不变。
func TestFleetForgetHandler_ConfirmRequired(t *testing.T) {
	h := newForgetHandlers()
	seedForgetMachine(h, "probe")
	req := httptest.NewRequest(http.MethodPost, "/api/fleet/forget", strings.NewReader(`{"machine":"probe"}`))
	w := httptest.NewRecorder()
	h.FleetForgetHandler(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("缺 confirm 应 400，实得 %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "CONFIRM_REQUIRED") {
		t.Fatalf("应报 CONFIRM_REQUIRED：%s", w.Body.String())
	}
	if h.Store.GetSnapshot("probe") == nil {
		t.Fatal("拒执不得动名册")
	}
}

// TestFleetForgetHandler_RealDeleteLandsInStore —— 带 confirm 的真删落到 store（只删点名那条）。
func TestFleetForgetHandler_RealDeleteLandsInStore(t *testing.T) {
	h := newForgetHandlers()
	seedForgetMachine(h, "probe")
	seedForgetMachine(h, "node-a")
	code, out := doFleetForget(t, h, `{"machine":"probe","confirm":true}`)
	if code != http.StatusOK {
		t.Fatalf("真删应 200，实得 %d（%v）", code, out)
	}
	if out["changed"] != true {
		t.Fatalf("在册机真删 changed 应为 true：%v", out)
	}
	if h.Store.GetSnapshot("probe") != nil {
		t.Fatal("真删后 probe 应离册")
	}
	if h.Store.GetSnapshot("node-a") == nil {
		t.Fatal("真删不得波及其他机器")
	}
}

// TestFleetForgetHandler_Idempotent —— 不在册的机再删 ⇒ 200 + changed=false（幂等，不是错误）。
func TestFleetForgetHandler_Idempotent(t *testing.T) {
	h := newForgetHandlers()
	code, out := doFleetForget(t, h, `{"machine":"ghost","confirm":true}`)
	if code != http.StatusOK {
		t.Fatalf("幂等重跑应 200，实得 %d", code)
	}
	if out["changed"] != false {
		t.Fatalf("不在册 changed 应为 false：%v", out)
	}
}

// TestFleetForgetHandler_BadInputs —— 空名 / 坏 JSON 一律 400。
func TestFleetForgetHandler_BadInputs(t *testing.T) {
	h := newForgetHandlers()
	if code, _ := doFleetForget(t, h, `{"machine":"   "}`); code != http.StatusBadRequest {
		t.Fatalf("空 machine 应 400，实得 %d", code)
	}
	if code, _ := doFleetForget(t, h, `not json`); code != http.StatusBadRequest {
		t.Fatalf("坏体应 400，实得 %d", code)
	}
}

// TestFleetForget_RouteSurface —— 两态：`main.go` 的**真实注册面**里有这条路由 ⇒ chi 认得（200）；
// 没有 ⇒ chi 当作未知路由（404）。★ 结论由扫真源驱动（照 routes_test.go 的口径），✗ 不硬编码。
func TestFleetForget_RouteSurface(t *testing.T) {
	src, err := os.ReadFile(mainGoPath(t))
	if err != nil {
		t.Skipf("读不到主控 main.go：%v", err)
	}
	registered := strings.Contains(string(src), `"/api/fleet/forget"`)

	r := chi.NewRouter()
	if registered {
		h := newForgetHandlers()
		seedForgetMachine(h, "probe")
		r.Post("/api/fleet/forget", h.FleetForgetHandler)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/fleet/forget",
		bytes.NewReader([]byte(`{"machine":"probe","dry_run":true}`)))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	t.Logf("POST /api/fleet/forget → %d (registered=%v)", w.Code, registered)

	if registered && w.Code != http.StatusOK {
		t.Fatalf("main.go 已注册 ⇒ 应 200，实得 %d", w.Code)
	}
	if !registered && w.Code != http.StatusNotFound {
		t.Fatalf("main.go 未注册 ⇒ 应 404（chi 未知路由），实得 %d", w.Code)
	}
}
