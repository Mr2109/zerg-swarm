package main

import "testing"

// ⑨ 收版账：全部缺口已闭时 export 必须照出件（缺口 `GAP-20261023-01`）。
//
// 病：旧判据把「未闭 0」一律判红 ⇒ 收口把缺口清空后，缺口总账这一件再也重生不了。
func TestGapExportAllClosedAllowed(t *testing.T) {
	if !gapExportAllClosedAllowed(1521, 0) {
		t.Fatal("真源非空且全部已闭 ⇒ 必须允许出收版账")
	}
	if gapExportAllClosedAllowed(0, 0) {
		t.Fatal("真源空 ⇒ 必须仍拒（那才是真的没得导）")
	}
	if gapExportAllClosedAllowed(1521, 3) {
		t.Fatal("真源非空但仍有未闭 ⇒ 不走「收版账」这一支（原有路径处理）")
	}
}
