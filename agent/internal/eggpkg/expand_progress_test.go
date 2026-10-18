// expand_progress_test.go —— 展开进度面的两态验收（GAP-20261018-15）。
//
// 钉住的语义（任一条被改坏，本文件先红）：
//
//	态① **展开中**：`.expanding` 是一份可读的 `ExpandProgress` JSON —— `state=expanding`、
//	      `bytes_total` == 段表 `DataBytes`、`segments_total` == 段数，且回调的 `bytes_done`
//	      单调不减、最后一帧正好落满；
//	态② **展开后**：`.expanding` 已收掉（不在展开中），`.complete` 在（完整性凭据仍只有它）；
//	幂等复用**零回归**：同代再来一次 ⇒ `Skipped=true`、一次进度回调都没有、**不写** `.expanding`。
package eggpkg

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestExpandEgg_ProgressFaceTwoStates(t *testing.T) {
	dir := t.TempDir()
	egg := filepath.Join(dir, "progress.egg")
	segs := []eggTestSeg{
		{Path: "weights/a.gguf", Data: bytes.Repeat([]byte{0xAB}, 64<<10), Mode: 0o644},
		{Path: "engine/lib/libzerg-fake.so", Data: bytes.Repeat([]byte{0xCD}, 32<<10), Mode: 0o755},
		{Path: "cfg/alias.gguf", Link: "weights/a.gguf"},
	}
	eggTestPack(t, egg, segs, 4096)
	ix, err := ReadEggIndex(egg)
	if err != nil {
		t.Fatalf("夹具蛋读不出来：%v", err)
	}
	dst := filepath.Join(dir, "out")

	var frames []ExpandProgress
	var mid *ExpandProgress
	res, err := ExpandEggWithProgress(egg, dst, func(p ExpandProgress) {
		frames = append(frames, p)
		if mid == nil {
			// 态①：回调那一刻，盘上的 `.expanding` 必须已经能读（别进程也看得到同一份读数）。
			pr, rerr := ReadExpandProgress(dst)
			if rerr != nil {
				t.Fatalf("展开中读不到 `.expanding` 的进度正文：%v", rerr)
			}
			mid = pr
		}
	})
	if err != nil {
		t.Fatalf("展开失败：%v", err)
	}
	if res.Skipped {
		t.Fatal("首次展开不该是跳过档")
	}
	if mid == nil {
		t.Fatal("一次进度回调都没收到（进度面等于没接线）")
	}
	// 态①：中间态的逐字读数
	if mid.State != "expanding" {
		t.Fatalf("展开中的 state 应逐字是 %q，实得 %q", "expanding", mid.State)
	}
	if mid.BytesTotal != ix.DataBytes {
		t.Fatalf("bytes_total 应等于段表数据字节 %d，实得 %d", ix.DataBytes, mid.BytesTotal)
	}
	if mid.SegmentsTotal != len(ix.Entries) {
		t.Fatalf("segments_total 应等于段数 %d，实得 %d", len(ix.Entries), mid.SegmentsTotal)
	}
	if mid.BytesDone <= 0 || mid.BytesDone > mid.BytesTotal {
		t.Fatalf("首帧 bytes_done 应在 (0, total] 内，实得 %d/%d", mid.BytesDone, mid.BytesTotal)
	}
	if mid.PID != os.Getpid() {
		t.Fatalf("进度件里应记展开者 pid=%d，实得 %d", os.Getpid(), mid.PID)
	}

	// 回调序列：字节单调不减、段数递增、最后一帧落满
	last := int64(-1)
	for i, f := range frames {
		if f.BytesDone < last {
			t.Fatalf("第 %d 帧 bytes_done 回退：%d → %d", i, last, f.BytesDone)
		}
		last = f.BytesDone
	}
	if last != ix.DataBytes {
		t.Fatalf("最后一帧应落满 %d 字节，实得 %d", ix.DataBytes, last)
	}
	if frames[len(frames)-1].SegmentsDone != len(ix.Entries) {
		t.Fatalf("最后一帧应落满 %d 段，实得 %d", len(ix.Entries), frames[len(frames)-1].SegmentsDone)
	}

	// 态②：展开完成后 —— `.expanding` 收掉（不在展开中）、`.complete` 在
	if pr, rerr := ReadExpandProgress(dst); rerr == nil {
		t.Fatalf("展开完成后 `.expanding` 应已收掉，实得仍在且 state=%q", pr.State)
	}
	if _, serr := os.Stat(filepath.Join(dst, eggMarkerComplete)); serr != nil {
		t.Fatalf("展开完成后 `.complete` 必须在，实得 %v", serr)
	}

	// 幂等复用零回归：同代再展开一次 ⇒ 跳过、零回调、不写 `.expanding`
	var again int
	res2, err := ExpandEggWithProgress(egg, dst, func(ExpandProgress) { again++ })
	if err != nil {
		t.Fatalf("幂等复用那次报错：%v", err)
	}
	if !res2.Skipped {
		t.Fatal("同代 `.complete` 在 ⇒ 必须走跳过档（幂等复用的那条 38 s 路径）")
	}
	if again != 0 {
		t.Fatalf("跳过档一次进度回调都不该有（它一个字节都没写），实得 %d 次", again)
	}
	if _, rerr := ReadExpandProgress(dst); rerr == nil {
		t.Fatal("跳过档不该写 `.expanding`（会把「不在展开」误报成「正在展开」）")
	}
}
