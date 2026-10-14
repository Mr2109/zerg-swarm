// dualwrite_test.go —— M1 双写仲裁器的定向单测（★ 一格两态 ＋ 两枚牙）。
//
// 一格两态（本件的主格）：**文件成功 ＋ 库写失败 ⇒ 文件面结果不变（返 nil）、落后量可读**；
//
//	反面态 = 文件失败 ⇒ 返文件错、库腿**一次都不调**、落后量**不动**。
//
// 两枚牙：① `TestDualWrite_FileLegFails_DBLegNeverCalled`（负控对象 = 态①那条「文件失败不调库」分支：
//
//	哑化它 ⇒ 本格必 FAIL）；② `TestDualWrite_BothOk_NoLag`（正控：两腿都成 ⇒ 返 nil 且落后量恒 0）。
package zdb

import (
	"errors"
	"strings"
	"testing"
)

// TestDualWrite_TwoStates —— ★ 一格两态（本单的核心交付格）。
//
//	态①：文件腿失败 ⇒ 返**文件错**、库腿一次不调、落后量不动（0）。
//	态②：文件成功 ＋ 库腿失败 ⇒ 返 **nil（文件面结果不变）**、落后量可读（Count=1 · LastErr 点名）。
func TestDualWrite_TwoStates(t *testing.T) {
	// ── 态①：文件腿失败 ──
	ResetDualWriteLag()
	fileErr := errors.New("文件腿写不动（卷不在）")
	dbCalls, fileCalls := 0, 0
	got := DualWrite(
		func() error { fileCalls++; return fileErr },
		func() error { dbCalls++; return nil },
	)
	if !errors.Is(got, fileErr) {
		t.Fatalf("态①：文件腿失败时调用方应拿到**文件错**（仲裁真源在文件），实得 %v", got)
	}
	if dbCalls != 0 {
		t.Fatalf("态①：文件腿已失败 ⇒ 库腿**一次都不该调**，实得 dbCalls=%d", dbCalls)
	}
	if fileCalls != 1 {
		t.Fatalf("态①：文件腿应被调恰 1 次，实得 %d", fileCalls)
	}
	if rep := DualWriteLag(); rep.Count != 0 || rep.LastErr != "" {
		t.Fatalf("态①：文件腿失败**不是**库落后（库腿根本没跑）⇒ 落后量应恒 0，实得 %+v", rep)
	}

	// ── 态②：文件成功 ＋ 库腿失败 ──
	ResetDualWriteLag()
	dbErr := errors.New("库腿炸（库落后量应记这一笔）")
	fileRC := 0 // 「文件面 rc」：由文件腿写、调用方看得见的结果
	got2 := DualWrite(
		func() error { fileRC = 0; return nil }, // 文件成功 ⇒ rc 不变（0）
		func() error { return dbErr },           // 库写失败
	)
	if got2 != nil {
		t.Fatalf("态②：文件腿成功 ⇒ 调用方结果**只由文件腿定**（应返 nil），实得 %v", got2)
	}
	if fileRC != 0 {
		t.Fatalf("态②：文件面 rc 不该被库腿失败改动，实得 %d", fileRC)
	}
	rep := DualWriteLag()
	if rep.Count != 1 {
		t.Fatalf("态②：库腿失败应记 1 笔落后量（可读），实得 Count=%d", rep.Count)
	}
	if rep.LastErr != dbErr.Error() {
		t.Fatalf("态②：落后量应点名最近一次库腿错误，实得 %q", rep.LastErr)
	}
	if rep.LastAt.IsZero() {
		t.Fatalf("态②：落后量应带时刻戳（LastAt）")
	}
}

// TestDualWrite_FileLegFails_DBLegNeverCalled —— ★ 牙①（负控对象：态①的「文件失败不调库」分支）。
// 用**计数间谍**逐次核：文件腿失败 ⇒ 库腿调用数为 0（哑化那条分支 ⇒ 本格必 FAIL）。
func TestDualWrite_FileLegFails_DBLegNeverCalled(t *testing.T) {
	for _, n := range []int{1, 3} {
		ResetDualWriteLag()
		dbCalls := 0
		for i := 0; i < n; i++ {
			err := DualWrite(
				func() error { return errors.New("文件腿失败") },
				func() error { dbCalls++; return nil },
			)
			if err == nil {
				t.Fatalf("第 %d 次：文件腿失败应返非 nil，实得 nil", i)
			}
		}
		if dbCalls != 0 {
			t.Fatalf("文件腿失败 %d 次 ⇒ 库腿调用数必须恒 0（文件没成、库不许独自前进），实得 %d", n, dbCalls)
		}
		if rep := DualWriteLag(); rep.Count != 0 {
			t.Fatalf("文件腿失败 %d 次 ⇒ 落后量应恒 0（库腿没跑），实得 %d", n, rep.Count)
		}
	}
}

// TestDualWrite_BothOk_NoLag —— ★ 正控牙②：两腿都成 ⇒ 返 nil、落后量恒 0（✗ 不许把成功也记成落后）。
func TestDualWrite_BothOk_NoLag(t *testing.T) {
	ResetDualWriteLag()
	fileCalls, dbCalls := 0, 0
	got := DualWrite(func() error { fileCalls++; return nil }, func() error { dbCalls++; return nil })
	if got != nil {
		t.Fatalf("两腿都成应返 nil，实得 %v", got)
	}
	if fileCalls != 1 || dbCalls != 1 {
		t.Fatalf("两腿各应被调恰 1 次，实得 file=%d db=%d", fileCalls, dbCalls)
	}
	if rep := DualWriteLag(); rep.Count != 0 || rep.LastErr != "" {
		t.Fatalf("两腿都成 ⇒ 落后量应恒 0／空，实得 %+v", rep)
	}
}

// TestDualWrite_LagAccumulates —— 落后量是**累计**（几笔没进库）且点名最近一笔。
func TestDualWrite_LagAccumulates(t *testing.T) {
	ResetDualWriteLag()
	for i := 0; i < 3; i++ {
		if err := DualWrite(func() error { return nil },
			func() error { return errors.New("库腿第 " + string(rune('A'+i)) + " 笔失败") }); err != nil {
			t.Fatalf("文件腿成功 ⇒ 不该返错，实得 %v", err)
		}
	}
	rep := DualWriteLag()
	if rep.Count != 3 {
		t.Fatalf("三笔库腿失败应累计 Count=3，实得 %d", rep.Count)
	}
	if !strings.Contains(rep.LastErr, "第 C 笔") {
		t.Fatalf("落后量应点名**最近**一笔（第 C 笔），实得 %q", rep.LastErr)
	}
}
