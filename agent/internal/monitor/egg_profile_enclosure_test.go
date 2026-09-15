// egg_profile_enclosure_test.go —— 卵档案 v2 的封闭等级（茧壁批 1；设计稿 §4.4 + §六 判据 8）。
//
// 三条硬口径，每条都有能红的版本：
//
//	① v1 旧档案必须**仍可读**，等级按 `unverified` 处理（既不当 `enclosed.*`，也不当拒绝）；
//	② v2 缺 enclosure 块 / 等级串非法 / 缺 checked_at ⇒ **拒**（fail-closed）；
//	③ `expected` 与 `observed` 两字段**都出现**，不相等时各自保留（不许合并成一个布尔）。
package monitor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/agent/internal/enclosure"
)

// v1ProfileYAML v1 旧档案在磁盘上的原文（六字段 + 溯源，**无 enclosure 块**）——
// 等于老机器上既有档案的形态：升版后必须照样读得进。
const v1ProfileYAML = `# X3 实测档案（v1：无封闭字段）
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

// v2ProfileYAML v2 档案原文：expected 与 observed **不相等**（配方期望 kernel、外部实读只到 os）。
const v2ProfileYAML = `schema_version: 2
machine: x3
measured_at: 2026-09-15T10:00:00Z
calib_runs: 3
weight_size_gb: 54.2
peak_gtt_gb: 32.70
peak_mem_gb: 40.0
load_seconds: 42
throughput_tok_s: 35.5
suggested_idle_unload_s: 600
enclosure:
  expected: enclosed.kernel
  observed: enclosed.os
  allowlist:
    - ipc:in-space
    - net:loopback
  checked_at: 2026-09-15T10:00:05Z
  note: 期望等级与实测等级不一致（两字段各自保留，不许合并）
`

// TestEggProfileEnclosure_V1ArchiveReadable_LevelUnverified
// 判据①：v1 旧档案读得进（不许因升版变砖），等级 = unverified（不是 enclosed.*、也不是拒）。
func TestEggProfileEnclosure_V1ArchiveReadable_LevelUnverified(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.yaml")
	if err := os.WriteFile(path, []byte(v1ProfileYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := LoadEggProfile(path)
	if err != nil {
		t.Fatalf("v1 旧档案必须仍可读（兼容读），却被拒: %v", err)
	}
	if got := p.EnclosureLevel(); got != enclosure.LevelUnverified {
		t.Fatalf("v1 旧档案（未申报封闭）等级应判 unverified，实得 %q", got)
	}
	if got := p.EnclosureExpected(); got != "" {
		t.Fatalf("v1 未申报期望等级，应回空串，实得 %q", got)
	}
	// unverified ≠ 拒绝（§4.2）：账够就必须照常放行，不许拿等级当拒孵理由
	if r := CanHatchWithProfile(true, 100, 100, p); !r.Ok {
		t.Fatalf("v1 档案 + 两账够 应放行（unverified 不等于拒绝），实得: %s", r.Reason)
	}
}

// TestEggProfileEnclosure_V2RoundTrip_ExpectedObservedSeparate
// 判据③（含 YAML 往返）：两个字段都读到、不相等就各自保留，实测等级取自 observed。
func TestEggProfileEnclosure_V2RoundTrip_ExpectedObservedSeparate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v2.yaml")
	if err := os.WriteFile(path, []byte(v2ProfileYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := LoadEggProfile(path)
	if err != nil {
		t.Fatalf("v2 档案（expected≠observed）应可读: %v", err)
	}
	if p.Enclosure == nil {
		t.Fatal("enclosure 块丢了（YAML 标签没对上）")
	}
	if p.EnclosureExpected() != enclosure.LevelKernel {
		t.Fatalf("expected 丢失/被覆盖：实得 %q", p.EnclosureExpected())
	}
	if p.EnclosureLevel() != enclosure.LevelOS {
		t.Fatalf("observed 丢失/被覆盖：实得 %q", p.EnclosureLevel())
	}
	if p.EnclosureExpected() == p.EnclosureLevel() {
		t.Fatal("本用例本就该不相等；相等说明两字段被合并了")
	}
	if len(p.Enclosure.Allowlist) != 2 {
		t.Fatalf("放行清单必须随等级一起落档：实得 %v", p.Enclosure.Allowlist)
	}
	if p.Enclosure.CheckedAt.IsZero() {
		t.Fatal("checked_at 丢失（核验有有效期，证据必须带时间）")
	}
}

// TestEggProfileEnclosure_V2MissingBlockRefused —— 判据②：v2 缺 enclosure 块 ⇒ 拒。
func TestEggProfileEnclosure_V2MissingBlockRefused(t *testing.T) {
	p := validProfile()
	p.Enclosure = nil
	err := p.Validate()
	if err == nil {
		t.Fatal("v2 缺 enclosure 块应被拒（fail-closed），却放行了")
	}
	if !strings.Contains(err.Error(), "enclosure") {
		t.Fatalf("报错应点名 enclosure，实得 %v", err)
	}
}

// TestEggProfileEnclosure_FailClosed —— 判据②的其余形态：等级串非法 / 缺核验时间。
func TestEggProfileEnclosure_FailClosed(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*EggProfile)
		wantSub string
	}{
		{"observed 是陌生等级串（宣称与实际不一致的典型起点）", func(p *EggProfile) {
			p.Enclosure.Observed = "trusted"
		}, "不是合法等级"},
		{"expected 是陌生等级串", func(p *EggProfile) {
			p.Enclosure.Expected = "sandboxed"
		}, "不是合法等级"},
		{"expected 为空（判据 8：两个字段都必须出现）", func(p *EggProfile) {
			p.Enclosure.Expected = ""
		}, "不是合法等级"},
		{"observed 为空（无证据不许落档）", func(p *EggProfile) {
			p.Enclosure.Observed = ""
		}, "不是合法等级"},
		{"缺 checked_at（核验有有效期）", func(p *EggProfile) {
			p.Enclosure.CheckedAt = time.Time{}
		}, "checked_at"},
	}
	for _, c := range cases {
		p := validProfile()
		c.mutate(&p)
		err := p.Validate()
		if err == nil {
			t.Fatalf("%s：应拒（fail-closed），却放行了", c.name)
		}
		if !strings.Contains(err.Error(), c.wantSub) {
			t.Fatalf("%s：报错应提到 %q，实得 %v", c.name, c.wantSub, err)
		}
	}
}

// TestEggProfileEnclosure_NeverEnclosedWithoutEvidence
// 兜底不变量（§4.2 / AR4SI）：拿不到证据（这里 = 等级串非法）时**永不给 `enclosed.*`**。
// 即便调用方拿到的是没过 Validate 的档案，也不许把"不知道"说成"封闭了"。
func TestEggProfileEnclosure_NeverEnclosedWithoutEvidence(t *testing.T) {
	p := validProfile()
	p.Enclosure.Observed = "trusted" // 绕过 Validate 直接喂给消费侧
	if got := p.EnclosureLevel(); got == enclosure.LevelKernel || got == enclosure.LevelOS {
		t.Fatalf("无证据（等级串非法）却给了 enclosed.*：%q", got)
	} else if got != enclosure.LevelUnverified {
		t.Fatalf("应落 unverified，实得 %q", got)
	}
	p.Enclosure = nil
	if got := p.EnclosureLevel(); got != enclosure.LevelUnverified {
		t.Fatalf("无 enclosure 块应落 unverified，实得 %q", got)
	}
}
