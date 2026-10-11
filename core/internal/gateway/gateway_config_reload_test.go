package gateway

import (
	"reflect"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/config"
)

// GAP-20261010-24：网关原持构造时的 config 快照（`gateway.go` 的 NewGateway 存 `cfg`），
// 而主控 `POST /api/config/reload` 只把 `api.Handlers.Config` 指向**新对象**
// （handlers.go ReloadConfigHandler: `h.Config = newCfg`）⇒ 网关的路由表 / 别名 / 机队节点面
// 永远停在启动那一刻（`model add` + reload 报 ok，网关面照样 `model not found`）。
//
// 修法照孵卵面先例 GAP-20260925-42（`ControlHandlers.SetConfigSource`）：加注入口
// `Gateway.SetConfigSource` + 读面走 `currentConfig()` 现读 —— 装配处（zerg-core main）传入
// `func() *config.FleetConfig { return handlers.Config }`。
//
// 反射调用 SetConfigSource：改前源无此方法 ⇒ 不注入、读面退回构造时快照（正是要复现的旧行为）。
// 改前跑本用例 ⇒ 红（新模型查不到）；改后 ⇒ 绿。本件不直接引用 currentConfig/SetConfigSource，
// 故改前也能编译（两态可对照）。
func TestGAP24_GatewaySeesReloadedConfig(t *testing.T) {
	oldCfg := &config.FleetConfig{
		Models: map[string][]config.ModelCandidate{"oldmodel": {{Host: "node-a", File: "/models/old.gguf"}}},
		Fleet:  map[string]config.FleetNode{"node-a": {Host: "127.0.0.1", Port: 1}},
	}
	newCfg := &config.FleetConfig{
		Models: map[string][]config.ModelCandidate{"newmodel": {{Host: "node-a", File: "/models/new.gguf"}}},
		Fleet:  map[string]config.FleetNode{"node-a": {Host: "127.0.0.1", Port: 1}},
	}
	g := NewGateway("tok", oldCfg, nil, nil)
	cur := oldCfg
	if m := reflect.ValueOf(g).MethodByName("SetConfigSource"); m.IsValid() {
		src := func() *config.FleetConfig { return cur }
		m.Call([]reflect.Value{reflect.ValueOf(src)})
	}
	cur = newCfg // 模拟 POST /api/config/reload：主控配置指针换成新对象

	// 走网关真实路由面（pickRouteHost 读的就是路由表）：reload 后**必须**认新模型。
	if _, err := g.pickRouteHost("newmodel", "node-a"); err != nil {
		t.Fatalf("reload 后网关路由面仍看不到新模型（读到的是启动快照）：%v", err)
	}
	// 负控：旧模型已从新表移除 —— 认新且**不认旧**，证明确实读的是 reload 后的现表，
	// 而非「新表旧表叠加」的假绿。
	if _, err := g.pickRouteHost("oldmodel", "node-a"); err == nil {
		t.Fatalf("reload 后网关仍认已被移除的旧模型 ⇒ 读的不是现表（假绿）")
	}
}
