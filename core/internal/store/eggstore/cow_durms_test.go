// cow_durms_test.go —— 缺口 GAP-20261019-02 的判据件。
//
// 病（改前实测）：`CowResult.DurMS` 恒为 0.000，而外部墙钟真有 39.755 ms。
// 根因：函数签名是**匿名返回值** `(CowResult, error)`，而计时写在 `defer` 里改的是**局部变量**
// `res` —— 返回值在 defer 之前就被复制走了 ⇒ 读数永远拿不到。
//
// 治法：两处（CloneWholeFile / CloneSegment）改**具名返回** `(res CowResult, err error)`，
// defer 直接改具名返回值 ⇒ 计时落到调用方手里。
//
// 判据（确定性，不靠真实耗时）：注入一个每次调用 +7ms 的时钟 ⇒ DurMS 必须是 7。
// 改前必红（0），改后必绿。
package eggstore

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func cowDurMSCase(t *testing.T, clone func(dst, src string, opt CowOptions) (CowResult, error)) {
	t.Helper()
	tick := time.Unix(0, 0)
	opt := CowOptions{Now: func() time.Time {
		tick = tick.Add(7 * time.Millisecond)
		return tick
	}}
	dir := t.TempDir()
	src := filepath.Join(dir, "a.bin")
	if err := os.WriteFile(src, make([]byte, 4096), 0o644); err != nil {
		t.Fatalf("夹具落盘失败：%v", err)
	}
	dst := filepath.Join(dir, "b.bin")

	res, err := clone(dst, src, opt)
	if err != nil {
		t.Fatalf("克隆失败：%v", err)
	}
	if res.DurMS != 7 {
		t.Fatalf("DurMS=%v，期望 7 —— 说明 defer 改的还是局部变量（读数恒 0 的老病）", res.DurMS)
	}
}

func TestCloneWholeFileDurMSLands(t *testing.T) {
	cowDurMSCase(t, CloneWholeFile)
}

func TestCloneSegmentDurMSLands(t *testing.T) {
	tick := time.Unix(0, 0)
	opt := CowOptions{Now: func() time.Time {
		tick = tick.Add(7 * time.Millisecond)
		return tick
	}}
	dir := t.TempDir()
	src := filepath.Join(dir, "a.bin")
	if err := os.WriteFile(src, make([]byte, 8192), 0o644); err != nil {
		t.Fatalf("夹具落盘失败：%v", err)
	}
	dst := filepath.Join(dir, "b.bin")

	res, err := CloneSegment(dst, src, ByteRange{Offset: 0, Length: 4096}, opt)
	if err != nil {
		t.Fatalf("段克隆失败：%v", err)
	}
	if res.DurMS != 7 {
		t.Fatalf("DurMS=%v，期望 7 —— defer 没落到返回值", res.DurMS)
	}
}
