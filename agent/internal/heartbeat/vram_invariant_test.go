package heartbeat

import "testing"

// TestBuildBody_VramFreeEqualsTotalMinusUsed —— `Q-214` 的心跳侧判据：三格同源。
//
// 心跳里 `vram_free_gb` 就是 `vram_total_gb − vram_used_gb`（`heartbeat.go` 同一段、同一次取样），
// 所以「used 与 free/total 不同源」这个形状一旦真出现，这条恒等式必然先断 ⇒ 在此钉死。
// 取值用 x3 2026-09-26 现跑 `/api/resources/ledger` 的真读数（total 125 / used 45.7 / free 79.3）。
func TestBuildBody_VramFreeEqualsTotalMinusUsed(t *testing.T) {
	cases := []struct {
		name              string
		used, total       float64
		wantFree, wantSum float64
	}{
		{"x3 现跑真读数（含 GTT）", 45.7, 125, 79.3, 125},
		{"GTT 未计入会断的形状（旧缺陷形态）", 0.2, 125, 124.8, 125},
		{"小机器整值", 7.5, 24, 16.5, 24},
	}
	for _, c := range cases {
		b := baseBackend()
		r := newTestRunner(b, fakeVram{used: c.used, total: c.total, known: true}, &fakeActive{})
		body := r.buildBody()

		used, okU := body["vram_used_gb"].(float64)
		total, okT := body["vram_total_gb"].(float64)
		free, okF := body["vram_free_gb"].(float64)
		if !okU || !okT || !okF {
			t.Fatalf("%s：三格必须齐（used/total/free），实得 %v / %v / %v", c.name, body["vram_used_gb"], body["vram_total_gb"], body["vram_free_gb"])
		}
		if free != c.wantFree {
			t.Errorf("%s：vram_free_gb 应为 total−used = %v，实得 %v", c.name, c.wantFree, free)
		}
		// ★ 同源铁律：三格出自同一次取样 ⇒ `used + free == total` 恒成立（任一路另取样即断）
		if s := used + free; s > c.wantSum+0.05 || s < c.wantSum-0.05 {
			t.Errorf("%s：used + free 应为 %v，实得 %v（三格不同源 ⇒ `Q-214` 的形状）", c.name, c.wantSum, s)
		}
		if used != c.used || total != c.total {
			t.Errorf("%s：used/total 必须逐字等于取样真值，实得 %v / %v", c.name, used, total)
		}
	}
}

// TestBuildBody_VramSharesOneSample —— 同源加固：used / total 只能来自**同一个** vramSource。
// 注入源的两格若来自两次读数，本测用「只认一次调用」的源把这件事变成可判定的。
func TestBuildBody_VramSharesOneSample(t *testing.T) {
	src := &countingVram{used: 45.7, total: 125}
	r := newTestRunner(baseBackend(), src, &fakeActive{})
	body := r.buildBody()

	if src.calls != 2 {
		// 心跳读 used 一次、total 一次 ⇒ 恰好 2；多了就说明有第二条取样路径参与
		t.Fatalf("显存取样调用数应为 2（used 一次 + total 一次），实得 %d；"+
			"若 >2 ⇒ 存在第二条采样路径（`Q-214` 的形状）", src.calls)
	}
	if body["vram_free_gb"] != body["vram_total_gb"].(float64)-body["vram_used_gb"].(float64) {
		t.Fatalf("free 必须由同一次取样的 total−used 得出：%v", body)
	}
}

type countingVram struct {
	used, total float64
	calls       int
}

func (c *countingVram) VramUsedGb() (float64, bool)  { c.calls++; return c.used, true }
func (c *countingVram) VramTotalGb() (float64, bool) { c.calls++; return c.total, true }
