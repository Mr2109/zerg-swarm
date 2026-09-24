package monitor

import (
	"os"
	"path/filepath"
	"testing"
)

// TestReadGttVram —— 统一内存（GTT）汇总：多卡求和 + 缺字段/空目录必须如实 ok=false。
// 背景（X3 实测 2026-09-25）：Strix Halo 核显的专用显存只有 1 GiB，模型跑在 GTT 里；
// 只看 VRAM 会把「装得下」误判成 no_fit ⇒ 必须把 GTT 计入。
func TestReadGttVram(t *testing.T) {
	root := t.TempDir()
	write := func(rel, val string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(val), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// 正控：两张卡 ⇒ 求和（GiB 换算前的字节数）
	write("card0/device/mem_info_gtt_total", "133143986176") // 124 GiB
	write("card0/device/mem_info_gtt_used", "27981787054")   // ≈26.06 GiB
	write("card1/device/mem_info_gtt_total", "1073741824")   // 1 GiB
	write("card1/device/mem_info_gtt_used", "107374182")     // 0.1 GiB
	used, total, ok := readGttVram(root)
	if !ok {
		t.Fatal("两张卡都有 GTT ⇒ 必须 ok=true")
	}
	if want := uint64(133143986176 + 1073741824); total != want {
		t.Fatalf("total want %d got %d", want, total)
	}
	if want := uint64(27981787054 + 107374182); used != want {
		t.Fatalf("used want %d got %d", want, used)
	}

	// 负控①：独显机器/非 AMD ⇒ 没有 GTT 字段 ⇒ 必须 ok=false（不能编 0）
	empty := t.TempDir()
	if _, _, ok := readGttVram(empty); ok {
		t.Fatal("无 GTT 字段时必须 ok=false（不许编造）")
	}
	// 负控②：有字段但 total=0 ⇒ 同样 ok=false
	root2 := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root2, "card0", "device"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root2, "card0", "device", "mem_info_gtt_total"), []byte("0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := readGttVram(root2); ok {
		t.Fatal("total=0 时必须 ok=false")
	}
	// 负控③：字段非数字 ⇒ 当 0 处理，不得 panic
	root3 := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root3, "card0", "device"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root3, "card0", "device", "mem_info_gtt_total"), []byte("N/A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := readGttVram(root3); ok {
		t.Fatal("非数字字段必须当 0 ⇒ ok=false")
	}
}
