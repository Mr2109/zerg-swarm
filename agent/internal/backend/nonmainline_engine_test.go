package backend

// nonmainline_engine_test.go —— P1 第三批：适配器声明「必须由非主线引擎承载」的接线验收。
//
// 语义（设计-子端沙箱化-20260914 §1.2 / §4.7 / 附录 C·C1）：
//   - 适配器实现 modeladapter.NonMainlineEngine 且返回 true ⇒ 卵声明（registry 条目）必须带 cmd:；
//   - 缺 cmd: ⇒ 拒孵（502 engine implementation missing），绝不静默退回主线 llama-server；
//   - 有 cmd: ⇒ 正常走 cmd 覆盖路径（既有行为不变）。
// 真实依据：主线 llama.cpp 不支持 example-moe-36b 架构（unknown model architecture），
// K2 必须经 IFM fork（/home/g01/llama-k2）+ run-k2.sh（清 LD_LIBRARY_PATH）。

import (
	"testing"

	"github.com/Mr2109/zerg-swarm/agent/internal/modeladapter"
	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
)

// TestNonMainline_K2AdapterDeclares 钉住「K2 适配器声明了必须非主线」这一事实本身——
// 若有人删掉 RequiresNonMainlineEngine，这条会先红。
func TestNonMainline_K2AdapterDeclares(t *testing.T) {
	adp := modeladapter.Dispatch("example-moe-36b-Test")
	nonMainline, ok := adp.(modeladapter.NonMainlineEngine)
	if !ok {
		t.Fatalf("K2 适配器未实现 NonMainlineEngine 接口（P1 回退？）")
	}
	if !nonMainline.RequiresNonMainlineEngine() {
		t.Fatalf("K2 适配器 RequiresNonMainlineEngine()=false，与附录 C·C1 取证矛盾")
	}
}

// TestNonMainline_MainlineAdaptersDoNotDeclare 主线架构（llama/qwen 等）不得误声明，
// 否则会把所有无 cmd: 的普通卵全部拒掉（回归保护）。
func TestNonMainline_MainlineAdaptersDoNotDeclare(t *testing.T) {
	for _, name := range []string{"Qwen3.8-Flash-Next", "GLM-5.3-Flash", "gemma-4-26B"} {
		adp := modeladapter.Dispatch(name)
		if nonMainline, ok := adp.(modeladapter.NonMainlineEngine); ok && nonMainline.RequiresNonMainlineEngine() {
			t.Errorf("%s 的适配器不应声明 RequiresNonMainlineEngine=true", name)
		}
	}
}

// TestNonMainline_MissingCmdRejected 核心：K2 卵声明缺 cmd: ⇒ doStart 返回 502 拒孵，
// 且进程清单里不得出现它（更不得静默起主线 llama-server）。
func TestNonMainline_MissingCmdRejected(t *testing.T) {
	m := newEvictTestManager(1, map[string]*subproc{})
	entry := &registry.ModelEntry{
		Backend: "llama-server",
		File:    "/data/models/k2/k2horizon-q4_k_m.gguf",
		MemGB:   24,
		// 故意不给 Cmd
	}
	resp, err := m.doStart("example-moe-36b-Test", entry)
	if err != nil {
		t.Fatalf("doStart 返回 err=%v（应走 502 响应而非 err）", err)
	}
	if got, _ := resp["status"].(int); got != 502 {
		t.Fatalf("缺 cmd: 的非主线卵应拒孵 502，got=%v resp=%v", got, resp)
	}
	if code, _ := resp["code"].(string); code != "engine implementation missing" {
		t.Fatalf("code 应为 engine implementation missing，got=%q", code)
	}
	if _, exists := m.procs["example-moe-36b-Test"]; exists {
		t.Fatalf("拒孵后进程清单里不得出现该模型")
	}
}

// TestNonMainline_CmdPresentPassesGuard 有 cmd: ⇒ 不触发拒孵（走既有 cmd 覆盖路径）。
// 只验「通过了守卫、不再因缺 cmd: 被拒」——spawn 本身在该 entry 下会因二进制不存在而
// 500，但那属于既有的启动失败路径，不属于本守卫。
func TestNonMainline_CmdPresentPassesGuard(t *testing.T) {
	m := newEvictTestManager(1, map[string]*subproc{})
	entry := &registry.ModelEntry{
		Backend: "llama-server",
		File:    "/data/models/k2/k2horizon-q4_k_m.gguf",
		MemGB:   24,
		Cmd:     "/home/g01/llama-k2/build-k2/bin/llama-server --port {port} -m {file}",
	}
	resp, err := m.doStart("example-moe-36b-Test", entry)
	if err != nil {
		t.Fatalf("doStart 返回 err=%v", err)
	}
	if got, _ := resp["status"].(int); got == 502 {
		t.Fatalf("有 cmd: 的卵不应被本守卫拒孵（502），resp=%v", resp)
	}
}
