// resident_test.go —— 驻留明细上报的回归测试。
//
// （ParsePortSpec 与未托管端口探测的用例已随该层一起退场——附录 C·C7。）
package backend

import (
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
)

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
