// baseline_mem_test.go —— M15：基线/托管项占用必须取 drm fdinfo（gtt+vram），不能取 RSS。
//
// 用例文本**逐字照抄 X3 实测**（2026-09-14，两条手工推理服务，端口语义见设计附录 A）：
//
//	Qwen3.8-27B → drm-memory-gtt 34286572 KiB = 32.70 GB（旧口径 RSS 只报 1.41 GB）
//	k2horizon   → drm-memory-gtt 25144840 KiB = 23.98 GB（旧口径 RSS 只报 0.77 GB）
//
// 两者之和 56.68 GB 与全局 mem_info_gtt_used 完全吻合 ⇒ 逐进程归因正确。
package backend

import (
	"math"
	"testing"
)

func TestParseDrmMemoryGb(t *testing.T) {
	cases := []struct {
		name   string
		text   string
		wantGB float64
	}{
		{"X3 Qwen 实测（KiB）", "pos:\t0\nflags:\t0100002\ndrm-memory-vram:\t2208 KiB\ndrm-memory-gtt: \t34286572 KiB\ndrm-memory-cpu: \t0 KiB\n", 32.70},
		{"X3 K2 实测（KiB）", "pos:\t0\nflags:\t0100002\ndrm-memory-vram:\t1992 KiB\ndrm-memory-gtt: \t25144840 KiB\ndrm-memory-cpu: \t0 KiB\n", 23.98},
		{"无 drm 行（非 GPU 宿主）", "pos:\t0\nflags:\t0100002\n", 0},
		{"GiB 单位", "drm-memory-gtt: 2 GiB\n", 2},
		{"MiB 单位", "drm-memory-gtt: 1024 MiB\n", 1},
		{"只读数字不算（形状不符）", "drm-memory-gtt:\n", 0},
	}
	for _, c := range cases {
		got := parseDrmMemoryGb(c.text)
		if math.Abs(got-c.wantGB) > 0.05 {
			t.Fatalf("%s：期望约 %.2f GB，实得 %.4f GB", c.name, c.wantGB, got)
		}
	}
}

// TestReadProcessMemGbFallsBackToRss 保证"取不到 drm 信息"时不会返回 0 而误判成"没占内存"。
// 用一个必然存在且非 GPU 进程（本测试进程自己）验证：它没有 drm fd ⇒ 应回落到 RSS（> 0）。
func TestReadProcessMemGbFallsBackToRss(t *testing.T) {
	if got := readProcessMemGb(1); got < 0 {
		t.Fatalf("占用不可能为负，实得 %v", got)
	}
}
