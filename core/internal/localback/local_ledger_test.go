package localback

// local_ledger_test.go —— 批 5：本机（local）账本与显存诚实的反例优先测试。
//
// 覆盖 #29（不得拿内存冒充显存）、#30（本机驻留明细由 LocalBackend 自身状态构造，口径同远程）
// 与 #33（显存/内存"采到就带值、拿不到就缺席"的两平台不变量：macOS 断言不变，Linux 采到真值时走带值分支）。

import (
	"runtime"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/resources"
)

// 本机已加载模型时：显存不可冒充（#29），且"采到/没采到"两条路径都必须自洽（#33）。
// macOS（Apple Silicon 统一内存）显存如实"未知"；Linux 装了 GPU 工具时采到真值就带值——
// 两种情况都在本用例内断言，不放宽。
func TestSnapshot_NoFakeVramWhenModelLoaded(t *testing.T) {
	lb := &LocalBackend{state: stateReady, file: "/models/example-35b-v2.gguf", memGB: 22, circuit: 3}
	snap := lb.Snapshot()
	if snap == nil {
		t.Fatal("快照不应 nil")
	}
	// 显存**任何情况下**都不得等同 RSS/内存量（#29 铁律；旧行为 snap.GpuUsedGb = memGB 已修）。
	if snap.GpuUsedGb == float64(snap.BackendRssGb) && snap.BackendRssGb != 0 {
		t.Fatalf("显存不得等同 RSS/内存量：gpu=%v rss=%v", snap.GpuUsedGb, snap.BackendRssGb)
	}
	// 两平台一致的不变量（#33）：known=false ⇒ 三值缺席 + gpu_used_gb=0；known=true ⇒ 带真值 + 非统一内存。
	// 不存在"半真"（有值而 known=false，或 known=true 而无值）。
	if snap.VramKnown {
		if snap.VramTotalGb <= 0 || snap.VramUsedGb < 0 || snap.VramFreeGb < 0 {
			t.Fatalf("known=true 必须带真值，实得 %v/%v/%v", snap.VramTotalGb, snap.VramUsedGb, snap.VramFreeGb)
		}
		if snap.VramUnified {
			t.Fatal("独立显存与统一内存互斥：known=true 时 vram_unified 必须为 false")
		}
	} else {
		if snap.GpuUsedGb != 0 {
			t.Fatalf("显存拿不到时必须为 0（缺席），实得 %v", snap.GpuUsedGb)
		}
		if snap.VramTotalGb != 0 || snap.VramUsedGb != 0 || snap.VramFreeGb != 0 {
			t.Fatalf("显存未知时三值必须缺席（为 0），实得 %v/%v/%v", snap.VramTotalGb, snap.VramUsedGb, snap.VramFreeGb)
		}
	}
	// 统一内存形态按平台如实断言（契约：**只有**统一内存平台才可标 vram_unified=true）。
	// darwin（Apple Silicon）显存即内存 → unified=true；其它平台本机采样不到独立显存 → 真未知
	// （unified=false，消费方走 fail-closed 估算）。把非统一平台标成 unified=true 会把"未知"
	// 当成"内存口径"误判（比未知更危险），故两侧都必须断言，不跳过、不放宽。
	if runtime.GOOS == "darwin" {
		if snap.VramKnown {
			t.Fatal("本机拿不到独立显存，vram_known 必须为 false（不冒充）")
		}
		if !snap.VramUnified {
			t.Fatal("darwin（Apple Silicon 统一内存）应标 vram_unified=true")
		}
	} else if snap.VramUnified {
		t.Fatalf("%s 非统一内存平台不得标 vram_unified=true", runtime.GOOS)
	}
	// 本机后端占用口径保留（旧行为不变）
	if snap.BackendRssGb != 22 {
		t.Fatalf("本机后端占用应保持 22（旧行为不变），实得 %v", snap.BackendRssGb)
	}
}

// #30：有驻留时 resident[] 非空且字段来自 LocalBackend 自身状态（口径同远程）。
func TestSnapshot_ResidentEntryFromOwnState(t *testing.T) {
	lb := &LocalBackend{state: stateReady, file: "/models/example-35b-v2.gguf", memGB: 22, circuit: 3}
	snap := lb.Snapshot()
	if len(snap.Resident) != 1 {
		t.Fatalf("有驻留时 resident[] 应为 1 项，实得 %v", snap.Resident)
	}
	r := snap.Resident[0]
	if r.Alias != "example-35b-v2" {
		t.Fatalf("别名应为文件名去后缀，实得 %q", r.Alias)
	}
	if r.File != "/models/example-35b-v2.gguf" {
		t.Fatalf("file 应为真实权重路径，实得 %q", r.File)
	}
	if r.State != resources.StateReady || r.MemGb != 22 || !r.Managed || r.Source != "localback" {
		t.Fatalf("驻留项字段不符: %+v", r)
	}
	// 本机不追踪 last-use / 并发（单槽：对裁决无意义）——如实留 0，不编造
	if r.LastUsedAgoS != 0 || r.ReqCount != 0 || r.RssGb != 0 {
		t.Fatalf("本机未采样的字段必须留 0（不编造）: %+v", r)
	}
}

// #30：无驻留（idle）时 resident[] 为空且不报错；显存同样如实未知。
func TestSnapshot_IdleHasNoResident(t *testing.T) {
	lb := &LocalBackend{state: stateIdle, circuit: 3}
	snap := lb.Snapshot()
	if snap == nil {
		t.Fatal("快照不应 nil（未加载也要如实返回）")
	}
	if len(snap.Resident) != 0 {
		t.Fatalf("无驻留时 resident[] 应为空，实得 %v", snap.Resident)
	}
	if len(snap.Models) != 0 {
		t.Fatalf("无驻留时 models 应为空，实得 %v", snap.Models)
	}
	if snap.VramKnown {
		// Linux + GPU 工具时才可能走到这里：拿到就必须带真值，不许 known=true 而无值。
		if snap.VramTotalGb <= 0 {
			t.Fatalf("known=true 必须带真值，实得 %v", snap.VramTotalGb)
		}
	}
	if runtime.GOOS == "darwin" && snap.VramKnown {
		t.Fatal("vram_known 应为 false") // 原断言：macOS 统一内存拿不到独立显存
	}
	if !snap.VramKnown && snap.GpuUsedGb != 0 {
		t.Fatalf("无模型时显存也应为 0，实得 %v", snap.GpuUsedGb)
	}
}

// 状态映射：loading/ready/broken（熔断）→ 账本状态如实（不美化）。
func TestResidentState_Mapping(t *testing.T) {
	cases := []struct{ in, want string }{
		{stateReady, resources.StateReady},
		{stateLoading, resources.StateLoading},
		{stateBroken, resources.StateCrashed},
		{stateIdle, resources.StateIdle},
		{"something-else", resources.StateIdle},
	}
	for _, c := range cases {
		if got := residentState(c.in); got != c.want {
			t.Fatalf("residentState(%q) want %q got %q", c.in, c.want, got)
		}
	}
}

// 模型名解析（账本别名/路由展示用）：去路径、去 .gguf/.GGUF 后缀。
func TestModelNameFromFile(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/models/example-35b-v2.gguf", "example-35b-v2"},
		{"/models/Qwen3.8-27B-Q4_K_M-vcruz305.GGUF", "Qwen3.8-27B-Q4_K_M-vcruz305"},
		{"plain-model", "plain-model"},
	}
	for _, c := range cases {
		if got := modelNameFromFile(c.in); got != c.want {
			t.Fatalf("modelNameFromFile(%q) want %q got %q", c.in, c.want, got)
		}
	}
}
