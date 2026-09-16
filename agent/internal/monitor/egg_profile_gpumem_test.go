package monitor

import (
	"strings"
	"testing"
)

// TestEggProfile_GpuMemKind —— 丙4（Mr2109 2026-09-16 拍）的守卫用例：
// 同一个 peak_gtt_gb 字段名在 macOS 装 **GPU wired**、在 Linux/Radeon 装 **GTT** ⇒
// 档案必须能说明"这是哪种账"，否则读档案的人会以为 macOS 也出了 GTT。
func TestEggProfile_GpuMemKind(t *testing.T) {
	base := validProfile() // 现成夹具（v2，带 enclosure 块）

	base.GpuMemKind = GpuMemKindWired
	if err := base.Validate(); err != nil {
		t.Fatalf("macOS wired 账应放行，实得：%v", err)
	}

	base.GpuMemKind = GpuMemKindGTT
	if err := base.Validate(); err != nil {
		t.Fatalf("Linux GTT 账应放行，实得：%v", err)
	}

	base.GpuMemKind = ""
	if err := base.Validate(); err != nil {
		t.Fatalf("未申报 kind（v1 老档案）应放行（兼容读），实得：%v", err)
	}

	base.GpuMemKind = "vram"
	err := base.Validate()
	if err == nil {
		t.Fatal("非法 kind 必须拒 —— 否则'哪种账'就含糊了")
	}
	if !strings.Contains(err.Error(), "gpu_mem_kind") {
		t.Fatalf("拒的理由要点名 gpu_mem_kind，实得：%v", err)
	}
}
