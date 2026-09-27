package monitor

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRocmVramFrom_SingleSource —— `Q-214` 的判据：used / total **必须同源折叠**。
//
// 背景（照实）：`Q-214` 报「x3 报 `vram_free_gb 124.8` / `vram_total_gb 125`，而 `vram_used_gb`
// 仍显 0.2（= 只有 VRAM，未含 GTT）」，怀疑心跳里 used / free 走了两条采样路径。
// 复核结论 = **不成立**（单一折叠，见本测 + 心跳侧 `TestBuildBody_VramFreeEqualsTotalMinusUsed`），
// 但「不同源」这个形状**没有机器判据挡着** ⇒ 本测把那一条钉死：
//
//	正控：GTT 在场 ⇒ used 必须含 GTT used（若谁把它拆成「VRAM-only used + 含 GTT 的 total」，
//	      本条当场红 —— 那正是 `0.2 / 124.8` 的形状）。
//	负控①：无 GTT（独显机器）⇒ used/total 与 VRAM 原值逐字相同（行为一字不变）。
//	负控②：GTT `total=0`/字段缺 ⇒ 不折叠（照 `readGttVram` 的 fail-closed）。
//	负控③：VRAM total ≤ 0 ⇒ `ok=false`（拿不到就如实说未知，不编）。
func TestRocmVramFrom_SingleSource(t *testing.T) {
	const gib = 1 << 30
	// 照 X3/Strix Halo 实测量级：专用显存 1 GiB（used 0.16），模型驻 GTT 124 GiB（used ≈26.06）。
	rocm := "GPU[0] : VRAM Total Memory (B): 1073741824\n" +
		"GPU[0] : VRAM Total Used Memory (B): 171798692\n"

	gtt := t.TempDir()
	dev := filepath.Join(gtt, "card0", "device")
	if err := os.MkdirAll(dev, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dev, "mem_info_gtt_total"), []byte("133143986176\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dev, "mem_info_gtt_used"), []byte("27981787054\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// ── 正控：GTT 计入 used 与 total（同一个折叠） ──────────────────────────────
	used, total, ok := rocmVramFrom(rocm, gtt)
	if !ok {
		t.Fatal("正控：VRAM total > 0 ⇒ 必须 ok=true")
	}
	wantUsed := float64(171798692+27981787054) / gib
	wantTotal := float64(1073741824+133143986176) / gib
	if diff := used - wantUsed; diff > 1e-6 || diff < -1e-6 {
		t.Fatalf("正控：used 应为 %f（VRAM used + GTT used），实得 %f", wantUsed, used)
	}
	if diff := total - wantTotal; diff > 1e-6 || diff < -1e-6 {
		t.Fatalf("正控：total 应为 %f（VRAM total + GTT total），实得 %f", wantTotal, total)
	}
	// ★ 有牙的那一句：used 必须**看得见 GTT**（绝不许是 VRAM-only 的 0.16）
	if used <= 1.0 {
		t.Fatalf("正控破（`Q-214` 的形状）：used=%f ⇒ 只报了 VRAM、没含 GTT —— "+
			"这正是「free 按含 GTT 的 total 算、used 只报 VRAM」那种自相矛盾读数", used)
	}
	// 心跳口径 = free := total − used ⇒ 三者同源时这条恒等式必须成立
	if free := total - used; free < 0 || free != wantTotal-wantUsed {
		t.Fatalf("正控破：free = total − used = %f（期望 %f）", free, wantTotal-wantUsed)
	}

	// ── 负控①：独显机器（没有 GTT 字段）⇒ 与原来一字不变 ───────────────────────
	used2, total2, ok2 := rocmVramFrom(rocm, t.TempDir())
	if !ok2 {
		t.Fatal("负控①：无 GTT 不该影响 ok（VRAM total > 0）")
	}
	if used2 != float64(171798692)/gib || total2 != float64(1073741824)/gib {
		t.Fatalf("负控①：无 GTT 时 used/total 必须等于 VRAM 原值，实得 %f / %f", used2, total2)
	}

	// ── 负控②：GTT total = 0 ⇒ 不折叠（fail-closed，不编 0 也不虚报） ──────────
	gtt0 := t.TempDir()
	dev0 := filepath.Join(gtt0, "card0", "device")
	if err := os.MkdirAll(dev0, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dev0, "mem_info_gtt_total"), []byte("0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dev0, "mem_info_gtt_used"), []byte("1024\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	used3, total3, _ := rocmVramFrom(rocm, gtt0)
	if used3 != float64(171798692)/gib || total3 != float64(1073741824)/gib {
		t.Fatalf("负控②：GTT total=0 时不许折叠，实得 %f / %f", used3, total3)
	}

	// ── 负控③：VRAM total ≤ 0 ⇒ ok=false（拿不到就如实说未知） ────────────────
	if _, _, ok4 := rocmVramFrom("GPU[0] : VRAM Total Memory (B): 0\n", gtt); ok4 {
		t.Fatal("负控③：VRAM total = 0 时必须 ok=false")
	}
}
