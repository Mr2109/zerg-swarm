// egg_profile_test.go —— 实测档案 + 双闸门测试。
//
// 对照锚点（§8.4 / 附录 A.3）：X3 注册表曾声明 Qwen3.8-27B mem_gb=18，
// 实测 GTT 32.70 GiB ⇒ 闸门若信声明值就翻车；本批判据函式就是这条错的根治手段。
package monitor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/agent/internal/enclosure"
)

// timeAt unix 秒 → UTC 时间（测试辅助）。
func timeAt(unix int64) time.Time { return time.Unix(unix, 0).UTC() }

// validProfile 返回一份过校验的档案（X3 Qwen 实测口径；**v2**：带 enclosure 块）。
func validProfile() EggProfile {
	p := validProfileV1()
	p.SchemaVersion = EggProfileSchemaVersion
	p.Enclosure = &enclosure.Verdict{
		Expected:  enclosure.LevelKernel,
		Observed:  enclosure.LevelKernel,
		Allowlist: []string{"ipc:in-space", "net:loopback"},
		CheckedAt: timeAt(1700000001),
	}
	return p
}

// validProfileV1 v1 旧档案（**无 enclosure 块**）：必须仍可读，等级按 unverified 处理。
func validProfileV1() EggProfile {
	return EggProfile{
		WeightSizeGb:         54.2,
		PeakGttGb:            32.70,
		PeakMemGb:            40.0,
		LoadSeconds:          42,
		ThroughputTokS:       35.5,
		SuggestedIdleUnloadS: 600,
		SchemaVersion:        EggProfileSchemaVersionV1,
		Machine:              "x3",
		MeasuredAt:           timeAt(1700000000),
		CalibRuns:            3,
	}
}

// ══════════════ 档案校验 ══════════════

func TestEggProfileValidate_OK(t *testing.T) {
	if err := validProfile().Validate(); err != nil {
		t.Fatalf("合法档案不应报错: %v", err)
	}
}

func TestEggProfileValidate_FailClosed(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*EggProfile)
		wantSub string
	}{
		{"缺 schema_version", func(p *EggProfile) { p.SchemaVersion = 0 }, "schema_version"},
		{"错版本", func(p *EggProfile) { p.SchemaVersion = 99 }, "schema_version"},
		{"缺 machine", func(p *EggProfile) { p.Machine = "" }, "machine"},
		{"缺 measured_at", func(p *EggProfile) { p.MeasuredAt = time.Time{} }, "measured_at"},
		{"标定轮数不足（铁律：三次取上界）", func(p *EggProfile) { p.CalibRuns = 2 }, "calib_runs"},
		{"缺权重体积", func(p *EggProfile) { p.WeightSizeGb = 0 }, "weight_size_gb"},
		{"缺峰值 GTT", func(p *EggProfile) { p.PeakGttGb = 0 }, "peak_gtt_gb"},
		{"缺峰值内存", func(p *EggProfile) { p.PeakMemGb = 0 }, "peak_mem_gb"},
		{"缺装载耗时", func(p *EggProfile) { p.LoadSeconds = 0 }, "load_seconds"},
		{"缺吞吐", func(p *EggProfile) { p.ThroughputTokS = 0 }, "throughput_tok_s"},
		{"缺空闲阈值", func(p *EggProfile) { p.SuggestedIdleUnloadS = 0 }, "suggested_idle_unload_s"},
	}
	for _, c := range cases {
		p := validProfile()
		c.mutate(&p)
		err := p.Validate()
		if err == nil {
			t.Fatalf("%s：应拒（fail-closed），却放行了", c.name)
		}
		if !strings.Contains(err.Error(), c.wantSub) {
			t.Fatalf("%s：报错应提到 %s，实得 %v", c.name, c.wantSub, err)
		}
	}
}

// TestEggProfileNeedCoeff —— need = 峰值 × 1.1（§8.4 第 2 步安全系数）。
func TestEggProfileNeedCoeff(t *testing.T) {
	p := validProfile()
	if !approx(p.GttNeed(), 32.70*1.1) {
		t.Fatalf("GttNeed 应为 35.97，实得 %.2f", p.GttNeed())
	}
	if !approx(p.MemNeed(), 44.0) {
		t.Fatalf("MemNeed 应为 44.0，实得 %.2f", p.MemNeed())
	}
}

// TestLoadEggProfile_RoundTrip —— 写 YAML → 读回 → 校验过。
func TestLoadEggProfile_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "qwen3.8-27b.profile.yaml")
	yamlText := `# X3 实测档案（标定铁律：凡数字必实测）
schema_version: 1
machine: x3
measured_at: 2026-09-15T10:00:00Z
calib_runs: 3
weight_size_gb: 54.2
peak_gtt_gb: 32.70
peak_mem_gb: 40.0
load_seconds: 42
throughput_tok_s: 35.5
suggested_idle_unload_s: 600
`
	if err := os.WriteFile(path, []byte(yamlText), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := LoadEggProfile(path)
	if err != nil {
		t.Fatalf("读合法档案失败: %v", err)
	}
	if !approx(p.PeakGttGb, 32.70) || p.Machine != "x3" || p.CalibRuns != 3 {
		t.Fatalf("回读内容错: %+v", p)
	}
}

// TestLoadEggProfile_MissingFile_Refuses —— 档案缺失必须报错（"没有实测档案 ⇒ 不许孵"）。
func TestLoadEggProfile_MissingFile_Refuses(t *testing.T) {
	_, err := LoadEggProfile(filepath.Join(t.TempDir(), "nope.yaml"))
	if err == nil {
		t.Fatal("档案缺失应报错")
	}
}

// TestLoadEggProfile_InvalidContent_Refuses —— 字段残缺的档案读回即拒。
func TestLoadEggProfile_InvalidContent_Refuses(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	// 声明值冒充实测的典型形态：calib_runs 缺失
	if err := os.WriteFile(path, []byte("schema_version: 1\nmachine: x3\npeak_gtt_gb: 18\npeak_mem_gb: 20\nload_seconds: 30\nthroughput_tok_s: 30\nsuggested_idle_unload_s: 600\nweight_size_gb: 54\nmeasured_at: 2026-09-15T10:00:00Z\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadEggProfile(path); err == nil {
		t.Fatal("calib_runs 缺失的档案应被拒（不许拿声明值凑数）")
	}
}

// ══════════════ 双闸门 ══════════════

// TestCheckDualGate_BothPass —— 两账都过 ⇒ 放行。
func TestCheckDualGate_BothPass(t *testing.T) {
	r := CheckDualGate(60, 60, 32.70, 40)
	if !r.Ok {
		t.Fatalf("应放行: %s", r.Reason)
	}
	if r.GttShortGb != 0 || r.MemShortGb != 0 {
		t.Fatalf("放行时差额应为 0: %+v", r)
	}
}

// TestCheckDualGate_GttOnlyShort —— 只 GTT 缺 ⇒ 拒 + 差额。
func TestCheckDualGate_GttOnlyShort(t *testing.T) {
	r := CheckDualGate(30, 60, 32.70, 40)
	if r.Ok {
		t.Fatal("GTT 缺 2.7 GB 不应放行")
	}
	if !approx(r.GttShortGb, 2.70) || r.MemShortGb != 0 {
		t.Fatalf("差额错: %+v", r)
	}
	if !strings.Contains(r.Reason, "GTT") {
		t.Fatalf("原因应点名 GTT: %s", r.Reason)
	}
}

// TestCheckDualGate_MemOnlyShort —— 只内存缺 ⇒ 拒。
func TestCheckDualGate_MemOnlyShort(t *testing.T) {
	r := CheckDualGate(60, 35, 32.70, 40)
	if r.Ok || !approx(r.MemShortGb, 5) || r.GttShortGb != 0 {
		t.Fatalf("应拒且内存缺 5 GB: %+v", r)
	}
	if !strings.Contains(r.Reason, "内存") {
		t.Fatalf("原因应点名内存: %s", r.Reason)
	}
}

// TestCheckDualGate_BothShort —— 两账都缺 ⇒ 原因两条都列。
func TestCheckDualGate_BothShort(t *testing.T) {
	r := CheckDualGate(10, 10, 32.70, 40)
	if r.Ok || r.GttShortGb == 0 || r.MemShortGb == 0 {
		t.Fatalf("两账都缺应双拒: %+v", r)
	}
	if !strings.Contains(r.Reason, "GTT") || !strings.Contains(r.Reason, "内存") {
		t.Fatalf("原因应两条都列: %s", r.Reason)
	}
}

// TestCheckDualGate_ExactBoundary —— 恰好相等 ⇒ 过（≥ 语义）。
func TestCheckDualGate_ExactBoundary(t *testing.T) {
	if !CheckDualGate(32.70, 40, 32.70, 40).Ok {
		t.Fatal("恰好相等应放行")
	}
}

// TestCheckDualGate_NegativeAvail —— 可用为负（采样异常）按不够处理，不修正不粉饰。
func TestCheckDualGate_NegativeAvail(t *testing.T) {
	r := CheckDualGate(-1, 60, 32.70, 40)
	if r.Ok {
		t.Fatal("负可用不得放行")
	}
	if !approx(r.GttShortGb, 33.70) {
		t.Fatalf("差额应如实反映负可用: %.2f", r.GttShortGb)
	}
}

// TestCheckDualGateWithReserve —— reserve 从两侧各扣；负 reserve 不放大可用。
func TestCheckDualGateWithReserve(t *testing.T) {
	// 60 − 5 = 55 ≥ 40 ⇒ 过
	if !CheckDualGateWithReserve(60, 60, 40, 40, 5).Ok {
		t.Fatal("扣 reserve 后仍够应放行")
	}
	// 60 − 25 = 35 < 40 ⇒ GTT 拒
	r := CheckDualGateWithReserve(60, 60, 40, 40, 25)
	if r.Ok || r.GttShortGb == 0 {
		t.Fatalf("reserve 扣穿应拒: %+v", r)
	}
	// 负 reserve 视为 0（不许变相放大可用）
	if !CheckDualGateWithReserve(41, 41, 40, 40, -100).Ok {
		t.Fatal("负 reserve 应视为 0 且放行")
	}
}

// TestCanHatchWithProfile —— 判据函式：档案缺失必拒；档案有效走双闸门。
func TestCanHatchWithProfile(t *testing.T) {
	p := validProfile()
	// 档案缺失（C6 根治点：声明值 18 GB 不参与）
	r := CanHatchWithProfile(false, 100, 100, EggProfile{})
	if r.Ok || !strings.Contains(r.Reason, "实测档案") {
		t.Fatalf("无档案应拒孵: %+v", r)
	}
	// 有档案 + 账够 ⇒ 放行（need = 32.70×1.1 = 35.97 ≤ 60）
	if !CanHatchWithProfile(true, 60, 60, p).Ok {
		t.Fatal("有档案且账够应放行")
	}
	// 有档案 + 账不够 ⇒ 拒（安全系数 1.1 之后 40 < 35.97 不成立 ⇒ 用 35 造缺）
	r = CanHatchWithProfile(true, 35, 60, p)
	if r.Ok {
		t.Fatalf("need=35.97 > avail=35 应拒: %+v", r)
	}
}
