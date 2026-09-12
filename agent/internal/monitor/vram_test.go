package monitor

import (
	"runtime"
	"testing"
)

// TestParseRocmVram —— rocm-smi --showmeminfo vram 输出解析（X3/AMD 路径）。
func TestParseRocmVram(t *testing.T) {
	out := "" +
		"======================= ROCm System Management Interface =======================\n" +
		"GPU[0]\t\t: VRAM Total Memory (B): 25753026560\n" +
		"GPU[0]\t\t: VRAM Total Used Memory (B): 8589934592\n" +
		"===============================================================================\n"

	used, total := parseRocmVram(out)
	if used != 8589934592 {
		t.Fatalf("used 应为 8589934592，实得 %d", used)
	}
	if total != 25753026560 {
		t.Fatalf("total 应为 25753026560，实得 %d（注意 'Used Memory' 是 'Memory' 的子串，不得串位）", total)
	}
}

// TestParseRocmVram_UsedBeforeTotal —— 行序颠倒也必须解析正确。
func TestParseRocmVram_UsedBeforeTotal(t *testing.T) {
	out := "GPU[0] : VRAM Total Used Memory (B): 1073741824\nGPU[0] : VRAM Total Memory (B): 8589934592\n"
	used, total := parseRocmVram(out)
	if used != 1073741824 || total != 8589934592 {
		t.Fatalf("行序颠倒时解析错: used=%d total=%d", used, total)
	}
}

// TestParseRocmVram_Garbage —— 反例：无显存信息时解析出 0（宁可未知，不编造）。
func TestParseRocmVram_Garbage(t *testing.T) {
	used, total := parseRocmVram("No GPU found\nsome noise 12345\n")
	if used != 0 || total != 0 {
		t.Fatalf("无有效行应解析出 0/0，实得 %d/%d", used, total)
	}
}

// TestSampler_VramUnknownHonest —— vramKnown=false 时两个 getter 都必须 ok=false（不得返回假 0 当真值）。
func TestSampler_VramUnknownHonest(t *testing.T) {
	s := &Sampler{}
	if _, ok := s.VramUsedGb(); ok {
		t.Fatal("显存未知时 VramUsedGb 必须 ok=false")
	}
	if _, ok := s.VramTotalGb(); ok {
		t.Fatal("显存未知时 VramTotalGb 必须 ok=false")
	}
	if s.VramKnown() {
		t.Fatal("显存未知时 VramKnown 应为 false")
	}
}

// TestSampler_VramKnownPassthrough —— vramKnown=true 时按采样值返回。
func TestSampler_VramKnownPassthrough(t *testing.T) {
	s := &Sampler{vramKnown: true, vramUsedGb: 7.5, vramTotalGb: 24}
	if used, ok := s.VramUsedGb(); !ok || used != 7.5 {
		t.Fatalf("应返回 7.5/true，实得 %v/%v", used, ok)
	}
	if total, ok := s.VramTotalGb(); !ok || total != 24 {
		t.Fatalf("应返回 24/true，实得 %v/%v", total, ok)
	}
}

// TestSampleVram_NoFabricationOnDarwin —— macOS（Apple Silicon 统一内存）**不得编造**显存，必须 ok=false。
func TestSampleVram_NoFabricationOnDarwin(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("仅在 macOS 上验证统一内存平台不编造显存")
	}
	if _, _, ok := sampleVram(); ok {
		t.Fatal("macOS 无独立显存额度，sampleVram 必须返回 ok=false（不得拿内存冒充显存）")
	}
}
