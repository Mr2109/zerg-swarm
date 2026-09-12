package api

// resources_pin_test.go —— POST /api/resources/pin|unpin 的反例优先测试。
//
// 红线（逐条有断言）：
//   - Q5：ttl_s<=0 一律 400（无 TTL 的 pin 等同内存泄漏）；
//   - 未托管绝不接管：未托管项（managed=false）拒绝，且**零调用**动作侧；
//   - 非驻留绝不启动：不在驻留清单里的模型拒绝，且**零调用**动作侧（注入假 manager 断言零调用）。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/config"
	"github.com/Mr2109/zerg-swarm/core/internal/resources"
	"github.com/Mr2109/zerg-swarm/core/internal/store"
	"github.com/go-chi/chi/v5"
)

// fakePinController 记录调用（红线断言：拒绝路径必须零调用）。
type fakePinController struct {
	pinCalls   []string
	unpinCalls []string
	err        error
}

func (f *fakePinController) PinResource(host, model string, ttlS int) error {
	f.pinCalls = append(f.pinCalls, host+"/"+model)
	return f.err
}

func (f *fakePinController) UnpinResource(host, model string) error {
	f.unpinCalls = append(f.unpinCalls, host+"/"+model)
	return f.err
}

// newPinTestRouterAndFake 造路由（同一套 AuthMiddleware）+ 注入假动作侧。
func newPinTestRouterAndFake(h *Handlers) (*chi.Mux, *fakePinController) {
	fake := &fakePinController{}
	h.ResourcePins = fake
	return newResourcesTestRouter(h), fake
}

// machineWithResident 写一台带驻留明细的机器快照。
func machineWithResident(t *testing.T, h *Handlers, machine string, entries ...resources.ResidentEntry) {
	t.Helper()
	h.Store.ReceiveHeartbeat(store.HeartbeatRequest{
		Machine:        machine,
		MemTotalGb:     128,
		MemAvailableGb: 120,
		VramKnown:      true,
		VramTotalGb:    24,
		VramFreeGb:     20,
		Resident:       entries,
	})
}

// ── Q5：ttl_s<=0 一律 400（且零调用） ────────────────────────────────────────

func TestResourcePin_TTLRequired(t *testing.T) {
	h, _ := newLedgerHandlers(t)
	machineWithResident(t, h, "x3", resources.ResidentEntry{Alias: "ornith", State: "ready", Managed: true})
	r, fake := newPinTestRouterAndFake(h)

	for _, ttl := range []string{"0", "-1", "-600"} {
		body := `{"host":"x3","model":"ornith","ttl_s":` + ttl + `}`
		w := doResReq(t, r, http.MethodPost, "/api/resources/pin", "test-token", body)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("ttl_s=%s 应 400，实得 %d body=%s", ttl, w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "PIN_TTL_REQUIRED") {
			t.Errorf("错误码应为 PIN_TTL_REQUIRED，实得 %s", w.Body.String())
		}
	}
	if len(fake.pinCalls) != 0 {
		t.Fatalf("非法 TTL 的拒绝路径必须零调用动作侧，实得 %v", fake.pinCalls)
	}
}

// ── 红线：pin 不在驻留清单里的模型 → 拒绝且零调用（绝不启动/接管） ──────────────

func TestResourcePin_NotResidentRejectedNoAction(t *testing.T) {
	h, _ := newLedgerHandlers(t)
	machineWithResident(t, h, "x3") // 驻留清单为空
	r, fake := newPinTestRouterAndFake(h)

	w := doResReq(t, r, http.MethodPost, "/api/resources/pin", "test-token",
		`{"host":"x3","model":"ghost","ttl_s":600}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("非驻留项应 409，实得 %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "MODEL_NOT_RESIDENT") {
		t.Errorf("错误码应为 MODEL_NOT_RESIDENT，实得 %s", w.Body.String())
	}
	if len(fake.pinCalls) != 0 {
		t.Fatalf("红线被破：非驻留项触发了动作侧调用 %v（等于启动/接管进程）", fake.pinCalls)
	}
}

// ── 红线（Q6）：未托管项拒绝且零调用（绝不接管手工进程） ─────────────────────

func TestResourcePin_UnmanagedRejectedNoAction(t *testing.T) {
	h, _ := newLedgerHandlers(t)
	machineWithResident(t, h, "x3", resources.ResidentEntry{Alias: "manual", State: "ready", Managed: false})
	r, fake := newPinTestRouterAndFake(h)

	w := doResReq(t, r, http.MethodPost, "/api/resources/pin", "test-token",
		`{"host":"x3","model":"manual","ttl_s":600}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("未托管项应 409，实得 %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "UNMANAGED_NOT_PINNABLE") {
		t.Errorf("错误码应为 UNMANAGED_NOT_PINNABLE，实得 %s", w.Body.String())
	}
	if len(fake.pinCalls) != 0 {
		t.Fatalf("红线被破：未托管项触发了动作侧调用 %v（等于接管手工进程）", fake.pinCalls)
	}
}

// ── 成功路径：已驻留且托管 → 转发一次（按别名寻址） ──────────────────────────

func TestResourcePin_SuccessCallsControllerOnce(t *testing.T) {
	h, _ := newLedgerHandlers(t)
	machineWithResident(t, h, "x3",
		resources.ResidentEntry{Digest: "sha256:x", Alias: "ornith", State: "ready", Managed: true})
	r, fake := newPinTestRouterAndFake(h)

	w := doResReq(t, r, http.MethodPost, "/api/resources/pin", "test-token",
		`{"host":"x3","model":"ornith","ttl_s":600}`)
	if w.Code != http.StatusOK {
		t.Fatalf("应 200，实得 %d body=%s", w.Code, w.Body.String())
	}
	if len(fake.pinCalls) != 1 || fake.pinCalls[0] != "x3/ornith" {
		t.Fatalf("应恰好转发一次且按别名寻址，实得 %v", fake.pinCalls)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["pinned"] != true || resp["ttl_s"] != float64(600) {
		t.Fatalf("响应应回显 pinned/ttl_s，实得 %v", resp)
	}
}

// ── 未知机器 → 404；unpin 同理的红线 ─────────────────────────────────────────

func TestResourcePin_UnknownMachine(t *testing.T) {
	h, _ := newLedgerHandlers(t)
	r, fake := newPinTestRouterAndFake(h)
	w := doResReq(t, r, http.MethodPost, "/api/resources/pin", "test-token",
		`{"host":"nowhere","model":"m","ttl_s":60}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("未知机器应 404，实得 %d body=%s", w.Code, w.Body.String())
	}
	if len(fake.pinCalls) != 0 {
		t.Fatalf("未知机器拒绝路径必须零调用，实得 %v", fake.pinCalls)
	}
}

func TestResourceUnpin_NotResidentRejectedNoAction(t *testing.T) {
	h, _ := newLedgerHandlers(t)
	machineWithResident(t, h, "x3")
	r, fake := newPinTestRouterAndFake(h)
	w := doResReq(t, r, http.MethodPost, "/api/resources/unpin", "test-token",
		`{"host":"x3","model":"ghost"}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("非驻留 unpin 应 409，实得 %d body=%s", w.Code, w.Body.String())
	}
	if len(fake.unpinCalls) != 0 {
		t.Fatalf("红线被破：非驻留 unpin 触发了动作侧调用 %v", fake.unpinCalls)
	}
}

func TestResourceUnpin_UnmanagedRejectedNoAction(t *testing.T) {
	h, _ := newLedgerHandlers(t)
	machineWithResident(t, h, "x3", resources.ResidentEntry{Alias: "manual", State: "ready", Managed: false})
	r, fake := newPinTestRouterAndFake(h)
	w := doResReq(t, r, http.MethodPost, "/api/resources/unpin", "test-token",
		`{"host":"x3","model":"manual"}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("未托管 unpin 应 409，实得 %d body=%s", w.Code, w.Body.String())
	}
	if len(fake.unpinCalls) != 0 {
		t.Fatalf("红线被破：未托管 unpin 触发了动作侧调用 %v", fake.unpinCalls)
	}
}

// ── 未接线（ResourcePins=nil）→ 503（如实承认，不假装成功） ────────────────────

func TestResourcePin_NotWiredReturns503(t *testing.T) {
	h, _ := newLedgerHandlers(t)
	machineWithResident(t, h, "x3", resources.ResidentEntry{Alias: "ornith", State: "ready", Managed: true})
	r := newResourcesTestRouter(h) // ResourcePins 保持 nil
	w := doResReq(t, r, http.MethodPost, "/api/resources/pin", "test-token",
		`{"host":"x3","model":"ornith","ttl_s":60}`)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("未接线应 503，实得 %d body=%s", w.Code, w.Body.String())
	}
}

// ── 默认动作侧（SubEndPinController）：转发到子端，地址解析/错误如实 ────────────

func TestSubEndPinController_ForwardsPin(t *testing.T) {
	var gotPath, gotModel, gotToken string
	var gotTTL int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotToken = r.Header.Get("X-Auth-Token")
		var b struct {
			Model string `json:"model"`
			TTLS  int    `json:"ttl_s"`
		}
		_ = json.NewDecoder(r.Body).Decode(&b)
		gotModel, gotTTL = b.Model, b.TTLS
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := &SubEndPinController{Token: "tok", BaseURL: func(host string) (string, bool) { return srv.URL, true }}
	if err := c.PinResource("x3", "ornith", 600); err != nil {
		t.Fatalf("转发应成功：%v", err)
	}
	if gotPath != "/pin" || gotModel != "ornith" || gotTTL != 600 || gotToken != "tok" {
		t.Fatalf("转发内容不对：path=%s model=%s ttl=%d token=%s", gotPath, gotModel, gotTTL, gotToken)
	}
}

func TestSubEndPinController_Non200IsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"not_resident"}`))
	}))
	defer srv.Close()
	c := &SubEndPinController{BaseURL: func(host string) (string, bool) { return srv.URL, true }}
	if err := c.PinResource("x3", "ghost", 60); err == nil {
		t.Fatal("子端非 200 必须如实报错，不能假装成功")
	}
}

func TestSubEndPinController_NoAddressForLocal(t *testing.T) {
	c := &SubEndPinController{Fleet: map[string]config.FleetNode{}}
	if err := c.PinResource("local", "ornith", 60); err == nil {
		t.Fatal("本机无独立子端必须明确报错")
	} else if !strings.Contains(err.Error(), "子端地址") {
		t.Fatalf("错误信息应说明无子端地址，实得 %v", err)
	}
}
