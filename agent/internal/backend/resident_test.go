package backend

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"reflect"
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
)

// ── ParsePortSpec ────────────────────────────────────────────────

func TestParsePortSpec(t *testing.T) {
	cases := []struct {
		in   string
		want []int
		n    int
	}{
		{"", nil, 0},
		{"8100", []int{8100}, 1},
		{"8100,8101", []int{8100, 8101}, 2},
		{"9000-9002", []int{9000, 9001, 9002}, 3},
		{"8100,9000-9001", []int{8100, 9000, 9001}, 3},
		{"8100,8100", []int{8100}, 1},           // 去重
		{"9002-9000", nil, 0},                   // 逆序区间忽略
		{"abc,70000,-5", nil, 0},                // 非法忽略
		{" 9000 - 9001 ", []int{9000, 9001}, 2}, // 容忍空白
	}
	for _, c := range cases {
		got := ParsePortSpec(c.in)
		if len(got) != c.n {
			t.Fatalf("ParsePortSpec(%q) 应 %d 个，实得 %v", c.in, c.n, got)
		}
		if c.want != nil && !reflect.DeepEqual(got, c.want) {
			t.Fatalf("ParsePortSpec(%q) want %v got %v", c.in, c.want, got)
		}
	}
}

// TestParsePortSpec_Cap —— 超大区间被截断（防意外铺满整段）。
func TestParsePortSpec_Cap(t *testing.T) {
	got := ParsePortSpec("1-65535")
	if len(got) == 0 || len(got) > 4096 {
		t.Fatalf("应被截断到 <=4096，实得 %d", len(got))
	}
}

// ── 未托管探测：只读、如实标注、不动别人 ────────────────────────

// TestUnmanagedListeners_FindsUnmanaged_ExcludesOwned —— 探测到未托管监听；本管理器自己的端口不算。
func TestUnmanagedListeners_FindsUnmanaged_ExcludesOwned(t *testing.T) {
	lnFree, portFree := listenLoopback(t)
	defer lnFree.Close()
	lnOwned, portOwned := listenLoopback(t)
	defer lnOwned.Close()

	mgr := NewManager(nil, "x3")
	// 把 portOwned 标成"本管理器托管"
	mgr.procs["managed-model"] = &subproc{port: portOwned, state: StateReady}

	got := mgr.UnmanagedListeners([]int{portFree, portOwned})
	if len(got) != 1 {
		t.Fatalf("应只报 1 个未托管监听（排除自有端口），实得 %d: %+v", len(got), got)
	}
	if got[0].Port != portFree {
		t.Fatalf("未托管端口应为 %d，实得 %d", portFree, got[0].Port)
	}
	if got[0].Managed {
		t.Fatalf("未托管项 managed 必须为 false，实得 %+v", got[0])
	}
	if got[0].Note == "" {
		t.Fatal("未托管项应带来源说明")
	}
}

// TestUnmanagedListeners_DoesNotTakeOver —— 反例：探测后监听仍在服务（绝不被接管/杀）。
func TestUnmanagedListeners_DoesNotTakeOver(t *testing.T) {
	ln, port := listenLoopback(t)
	defer ln.Close()

	mgr := NewManager(nil, "x3")
	_ = mgr.UnmanagedListeners([]int{port})

	// 探测之后该监听必须仍然可用
	accepted := make(chan error, 1)
	go func() {
		c, err := ln.Accept()
		if err == nil {
			c.Close()
		}
		accepted <- err
	}()

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
	if err != nil {
		t.Fatalf("探测后监听端口不可用——说明被接管/杀了（违反 Q6）: %v", err)
	}
	conn.Close()
	select {
	case err := <-accepted:
		if err != nil {
			t.Fatalf("探测后 Accept 失败: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("探测后 Accept 未返回")
	}

	// 探测不得改动托管清单
	if len(mgr.procs) != 0 {
		t.Fatalf("探测不该写入任何托管项，实得 %d", len(mgr.procs))
	}
}

// TestUnmanagedListeners_EmptyPorts —— 空端口清单不探测、返回空。
func TestUnmanagedListeners_EmptyPorts(t *testing.T) {
	mgr := NewManager(nil, "x3")
	if got := mgr.UnmanagedListeners(nil); len(got) != 0 {
		t.Fatalf("空端口清单应返回空，实得 %v", got)
	}
}

// ── 驻留明细 ──────────────────────────────────────────────────────

// TestResidentDetail_ManagedFields —— 托管项如实带 状态/最后使用/在飞/实测RSS/声明内存/上下文/来源。
func TestResidentDetail_ManagedFields(t *testing.T) {
	mgr := NewManager(nil, "x3")
	entry := &registry.ModelEntry{
		File:  "/data/models/ornith.gguf",
		MemGB: 22,
		Custom: map[string]interface{}{
			"ctx_window": float64(262144),
			"digest":     "sha256-abc",
		},
	}
	sp := &subproc{
		model:    "example-35b",
		entry:    entry,
		state:    StateReady,
		reqCount: 2,
		lastUsed: time.Now().Add(-5 * time.Second),
		// 用当前进程当"后端进程"，避免起真进程
		proc: &exec.Cmd{Process: &os.Process{Pid: os.Getpid()}},
	}
	mgr.procs["example-35b"] = sp

	got := mgr.ResidentDetail()
	if len(got) != 1 {
		t.Fatalf("应有 1 项驻留明细，实得 %d", len(got))
	}
	d := got[0]
	if d.Alias != "example-35b" || d.State != StateReady {
		t.Fatalf("别名/状态不对: %+v", d)
	}
	if !d.Managed || d.Source != "managed" {
		t.Fatalf("托管项应 managed=true/source=managed，实得 %+v", d)
	}
	if d.ReqCount != 2 {
		t.Fatalf("在飞请求数应为 2，实得 %d", d.ReqCount)
	}
	if d.LastUsedAgoS < 4 || d.LastUsedAgoS > 60 {
		t.Fatalf("last_used_ago_s 应约 5，实得 %v", d.LastUsedAgoS)
	}
	if d.MemGb != 22 || d.CtxWindow != 262144 || d.Digest != "sha256-abc" || d.File == "" {
		t.Fatalf("声明内存/上下文/摘要/路径不全: %+v", d)
	}
	if d.RssGb <= 0 {
		t.Fatalf("实测 RSS 应 >0（当前进程）: %+v", d)
	}
}

// TestResidentDetail_NoFabrication —— 注册条目缺摘要/上下文时如实缺席（不编造）。
func TestResidentDetail_NoFabrication(t *testing.T) {
	mgr := NewManager(nil, "x3")
	mgr.procs["plain"] = &subproc{
		model:    "plain",
		entry:    &registry.ModelEntry{File: "/x.gguf"},
		state:    StateReady,
		lastUsed: time.Now(),
	}
	d := mgr.ResidentDetail()[0]
	if d.Digest != "" {
		t.Fatalf("无摘要来源时不得编造 digest，实得 %q", d.Digest)
	}
	if d.CtxWindow != 0 {
		t.Fatalf("无上下文来源时不得编造 ctx_window，实得 %d", d.CtxWindow)
	}
	if d.RssGb != 0 {
		t.Fatalf("无进程时 RSS 应为 0（缺席），实得 %v", d.RssGb)
	}
	if !d.Managed || d.Source != "managed" {
		t.Fatalf("托管项标注不对: %+v", d)
	}
}

// TestResidentDetail_SortedAndEmpty —— 多驻留项按别名排序；无驻留返回空。
func TestResidentDetail_SortedAndEmpty(t *testing.T) {
	mgr := NewManager(nil, "x3")
	if got := mgr.ResidentDetail(); len(got) != 0 {
		t.Fatalf("无驻留应为空，实得 %v", got)
	}
	mgr.procs["z"] = &subproc{model: "z", state: StateReady, lastUsed: time.Now()}
	mgr.procs["a"] = &subproc{model: "a", state: StateReady, lastUsed: time.Now()}
	got := mgr.ResidentDetail()
	if len(got) != 2 || got[0].Alias != "a" || got[1].Alias != "z" {
		t.Fatalf("应按别名排序，实得 %+v", got)
	}
}

// listenLoopback 起一个只绑 127.0.0.1 的临时监听，返回它和端口。
func listenLoopback(t *testing.T) (net.Listener, int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("起临时监听失败: %v", err)
	}
	return ln, ln.Addr().(*net.TCPAddr).Port
}
