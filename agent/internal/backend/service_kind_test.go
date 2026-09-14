// service_kind_test.go —— P1「服务类型互斥」的回归测试（llama xor ds4）。
//
// 设计依据：docs/01-设计/设计-子端服务切换与基线服务声明-20260914.md §7 S1/S2、§11 M6。
// 覆盖两类断言：
//  1. 纯函数 serviceKind / kindFromExecutable 的判定口径（含 cmd: 覆盖与未知可执行名）；
//  2. evictOtherKindsLocked 的动作：异类可动作⇒卸下、同类⇒保留、
//     异类但触红线（在飞 reqCount>0 / pin 未到期）⇒不硬来而计入 blocked。
package backend

import (
	"reflect"
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
)

// ── serviceKind：判定口径 ───────────────────────────────────────────────────

func TestServiceKind_ByBackend(t *testing.T) {
	cases := []struct {
		name  string
		entry *registry.ModelEntry
		want  string
	}{
		{"ds4 后端", &registry.ModelEntry{Backend: "ds4-server"}, kindDS4},
		{"llama 后端（缺省）", &registry.ModelEntry{Backend: "llama-server"}, kindLlama},
		{"空后端按 llama", &registry.ModelEntry{}, kindLlama},
		{"nil 条目按 llama", nil, kindLlama},
	}
	for _, c := range cases {
		if got := serviceKind(c.entry); got != c.want {
			t.Errorf("%s: serviceKind=%q，期望 %q", c.name, got, c.want)
		}
	}
}

func TestServiceKind_CmdOverrideWins(t *testing.T) {
	// cmd: 覆盖后端声明——以「实际会执行的可执行文件」为准。
	cases := []struct {
		name string
		cmd  string
		be   string
		want string
	}{
		{"cmd 是 ds4 绝对路径", "/home/g01/ds4-server --rocm", "llama-server", kindDS4},
		{"cmd 是 llama 路径", "/usr/local/bin/llama-server -m x", "ds4-server", kindLlama},
		{"cmd 含 fork 名也认 ds4", "/opt/ds4-pr670/ds4-server", "", kindDS4},
		{"cmd 只有空白 ⇒ 回落后端", "   ", "ds4-server", kindDS4},
	}
	for _, c := range cases {
		e := &registry.ModelEntry{Backend: c.be, Cmd: registry.CmdString(c.cmd)}
		if got := serviceKind(e); got != c.want {
			t.Errorf("%s: serviceKind=%q，期望 %q", c.name, got, c.want)
		}
	}
}

func TestKindFromExecutable(t *testing.T) {
	cases := map[string]string{
		"ds4-server":      kindDS4,
		"/a/b/DS4-Server": kindDS4,
		"llama-server":    kindLlama,
		"/home/g01/llama-k2/build-k2/bin/llama-server": kindLlama,
		"":             kindLlama,
		"weird-binary": kindLlama,
	}
	for in, want := range cases {
		if got := kindFromExecutable(in); got != want {
			t.Errorf("kindFromExecutable(%q)=%q，期望 %q", in, got, want)
		}
	}
}

// ── evictOtherKindsLocked：动作与红线 ───────────────────────────────────────

func resident(name, backend string, reqCount int, pinFor time.Duration) *subproc {
	sp := &subproc{
		model:    name,
		state:    StateReady,
		entry:    &registry.ModelEntry{Backend: backend, MemGB: 20},
		lastUsed: time.Now().Add(-time.Hour),
		reqCount: reqCount,
	}
	if pinFor > 0 {
		sp.pinUntil = time.Now().Add(pinFor)
	}
	return sp
}

func TestEvictOtherKinds_LlamaResidentDS4Requested(t *testing.T) {
	m := newEvictTestManager(3, map[string]*subproc{
		"qwen-local": resident("qwen-local", "llama-server", 0, 0),
		"glm-new":    resident("glm-new", "ds4-server", 0, 0), // 同类：请求的就是 ds4
	})
	ev, blocked := m.evictOtherKindsLocked("glm-new", kindDS4)
	if want := []string{"qwen-local"}; !reflect.DeepEqual(ev, want) {
		t.Fatalf("应卸下异类 %v，实得 %v", want, ev)
	}
	if len(blocked) != 0 {
		t.Fatalf("不该有 blocked，实得 %v", blocked)
	}
	if _, still := m.procs["qwen-local"]; still {
		t.Fatal("异类驻留未被卸下（llama 与 ds4 同时在跑）")
	}
	if _, ok := m.procs["glm-new"]; !ok {
		t.Fatal("同类驻留被误卸")
	}
}

func TestEvictOtherKinds_SameKindUntouched(t *testing.T) {
	m := newEvictTestManager(3, map[string]*subproc{
		"a": resident("a", "ds4-server", 0, 0),
		"b": resident("b", "ds4-server", 0, 0),
	})
	ev, blocked := m.evictOtherKindsLocked("c", kindDS4)
	if len(ev) != 0 || len(blocked) != 0 {
		t.Fatalf("同类不应有任何动作，实得 ev=%v blocked=%v", ev, blocked)
	}
	if len(m.procs) != 2 {
		t.Fatalf("同类驻留被动过：现有 %d 个", len(m.procs))
	}
}

func TestEvictOtherKinds_RedLineInFlightBlocked(t *testing.T) {
	// 红线①：异类但有在飞请求 ⇒ 不卸，计入 blocked（由调用方拒装，绝不硬来）。
	m := newEvictTestManager(3, map[string]*subproc{
		"busy-llama": resident("busy-llama", "llama-server", 1, 0),
	})
	ev, blocked := m.evictOtherKindsLocked("new-ds4", kindDS4)
	if len(ev) != 0 {
		t.Fatalf("在飞异类不该被卸，实得 ev=%v", ev)
	}
	if want := []string{"busy-llama"}; !reflect.DeepEqual(blocked, want) {
		t.Fatalf("应把在飞异类计入 blocked=%v，实得 %v", want, blocked)
	}
	if _, still := m.procs["busy-llama"]; !still {
		t.Fatal("红线①被破：在飞的异类驻留被卸了")
	}
}

func TestEvictOtherKinds_RedLinePinBlocked(t *testing.T) {
	// 红线③：pin 未到期的异类同样不卸，计入 blocked。
	m := newEvictTestManager(3, map[string]*subproc{
		"pinned-llama": resident("pinned-llama", "llama-server", 0, time.Hour),
	})
	ev, blocked := m.evictOtherKindsLocked("new-ds4", kindDS4)
	if len(ev) != 0 {
		t.Fatalf("pin 未到期的异类不该被卸，实得 ev=%v", ev)
	}
	if want := []string{"pinned-llama"}; !reflect.DeepEqual(blocked, want) {
		t.Fatalf("应把 pin 异类计入 blocked=%v，实得 %v", want, blocked)
	}
}

func TestEvictOtherKinds_MixedActionableAndBlocked(t *testing.T) {
	// 混合：一个可动作、一个在飞 ⇒ 卸前者、报后者（宁可拒装也不并存两类服务）。
	m := newEvictTestManager(4, map[string]*subproc{
		"idle-llama": resident("idle-llama", "llama-server", 0, 0),
		"busy-llama": resident("busy-llama", "llama-server", 2, 0),
	})
	ev, blocked := m.evictOtherKindsLocked("new-ds4", kindDS4)
	if want := []string{"idle-llama"}; !reflect.DeepEqual(ev, want) {
		t.Fatalf("可动作异类应被卸 %v，实得 %v", want, ev)
	}
	if want := []string{"busy-llama"}; !reflect.DeepEqual(blocked, want) {
		t.Fatalf("在飞异类应计入 blocked %v，实得 %v", want, blocked)
	}
}

func TestEvictOtherKinds_ExceptsItself(t *testing.T) {
	// 目标模型自己（已在 procs 里，例如重载）不该被当成异类来卸。
	m := newEvictTestManager(3, map[string]*subproc{
		"me": resident("me", "llama-server", 0, 0),
	})
	ev, blocked := m.evictOtherKindsLocked("me", kindDS4)
	if len(ev) != 0 || len(blocked) != 0 {
		t.Fatalf("目标模型自身不该被处理，实得 ev=%v blocked=%v", ev, blocked)
	}
}
