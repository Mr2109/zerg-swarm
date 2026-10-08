package api

import (
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/config"
)

// GAP-20260925-42：config reload 换掉主控配置指针后，孵卵面（ControlHandlers）必须同刻看到新模型，
// 不得再 404 MODEL_NOT_FOUND。改前：构造时值拷贝快照 ⇒ reload 后仍看不到。
// （反射调用 SetConfigSource：改前源无此方法 ⇒ 不注入、退化为快照，正是要复现的旧行为。）
func TestGAP42_ControlSeesReloadedModels(t *testing.T) {
	oldCfg := &config.FleetConfig{
		Models: map[string][]config.ModelCandidate{"oldmodel": {{Host: "node-a"}}},
		Fleet:  map[string]config.FleetNode{"node-a": {Host: "127.0.0.1", Port: 1}},
	}
	newCfg := &config.FleetConfig{
		Models: map[string][]config.ModelCandidate{"newmodel": {{Host: "node-a"}}},
		Fleet:  map[string]config.FleetNode{"node-a": {Host: "127.0.0.1", Port: 1}},
	}
	h := NewControlHandlers("tok", oldCfg)
	cur := oldCfg
	if m := reflect.ValueOf(h).MethodByName("SetConfigSource"); m.IsValid() {
		src := func() *config.FleetConfig { return cur }
		m.Call([]reflect.Value{reflect.ValueOf(src)})
	}
	cur = newCfg // 模拟 POST /api/config/reload：主控配置指针换成新对象

	req := httptest.NewRequest("POST", "/api/control/load",
		strings.NewReader(`{"machine":"node-a","model":"newmodel"}`))
	w := httptest.NewRecorder()
	h.LoadHandler(w, req)
	body := w.Body.String()
	if w.Code == 404 || strings.Contains(body, "MODEL_NOT_FOUND") {
		t.Fatalf("reload 后孵卵面仍看不到新模型：code=%d body=%s", w.Code, body)
	}
}
