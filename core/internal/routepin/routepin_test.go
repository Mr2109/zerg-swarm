package routepin

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// routepin_test.go —— 覆盖表那一件的判据（**全离线**，每条自带 `t.TempDir()` 隔离）。
//
// 判三条（对应设计稿的三条口径）：
//
//	① **缺件 ≠ 坏件**：件不在盘 ⇒ 空表 + nil（「没有覆盖」是正常态）；坏件 / 主号不认 ⇒ error（不猜）。
//	② **TTL 是必需品**：`Active` 只看 `expires_at`；读不出时刻 ⇒ **不生效**（不许当永不过期）。
//	③ **读者不删行**：`Load` 一个字节都不写（写前/写后 sha256 + mtime 逐字比）。

func tmpTable(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "route_pins.json")
}

// TestLoad_MissingIsEmptyNotError —— 缺件 ⇒ 空表 + nil（「没有覆盖」是正常态，不是错误）。
func TestLoad_MissingIsEmptyNotError(t *testing.T) {
	p := tmpTable(t)
	tbl, err := Load(p)
	if err != nil {
		t.Fatalf("缺件不该报错：%v", err)
	}
	if len(tbl.Pins) != 0 {
		t.Fatalf("缺件的表该是空的，实得 %d 条", len(tbl.Pins))
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("`Load` 绝不该把件建出来（读者只读）")
	}
}

// TestLoad_BrokenIsError —— 坏件 / 主号不认 ⇒ error（**不猜、不静默降级**）。
func TestLoad_BrokenIsError(t *testing.T) {
	p := tmpTable(t)
	if err := os.WriteFile(p, []byte("{ 这不是 JSON"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(p); err == nil {
		t.Fatal("坏件本该报错")
	}
	if err := os.WriteFile(p, []byte(`{"id":"route-pins.v9","pins":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(p); err == nil {
		t.Fatal("不认的形制主号本该报错")
	}
}

// TestSaveLoad_RoundTrip —— 写 → 读 → 逐格同；同一个模型钉两次 = **覆盖**（不叠行）。
func TestSaveLoad_RoundTrip(t *testing.T) {
	p := tmpTable(t)
	tbl := &Table{ID: TableID}
	now := time.Now()
	a := Pin{Model: "m1", Machine: "x3", CreatedAt: now.Format(time.RFC3339),
		ExpiresAt: now.Add(2 * time.Hour).Format(time.RFC3339), TTLSeconds: 7200, By: "甲"}
	b := Pin{Model: "m1", Machine: "Mr2109", CreatedAt: now.Format(time.RFC3339),
		ExpiresAt: now.Add(time.Hour).Format(time.RFC3339), TTLSeconds: 3600, By: "乙"}
	tbl.Upsert(a)
	tbl.Upsert(b) // 同一个模型 ⇒ 覆盖
	if len(tbl.Pins) != 1 {
		t.Fatalf("同一个模型该只留一行（钉两次 = 覆盖），实得 %d 行", len(tbl.Pins))
	}
	if err := tbl.Save(p); err != nil {
		t.Fatalf("写不进：%v", err)
	}
	back, err := Load(p)
	if err != nil {
		t.Fatalf("读不回：%v", err)
	}
	if len(back.Pins) != 1 || back.Pins[0].Machine != "Mr2109" {
		t.Fatalf("写后读回不是刚落下的那一行：%+v", back.Pins)
	}
	if _, ok := back.HostFor("m1", now); !ok {
		t.Fatal("刚落下的行该命中")
	}
	if _, ok := back.HostFor("别的模型", now); ok {
		t.Fatal("没钉的模型不许命中")
	}
}

// TestActive_TTL —— 到期即不生效；**读不出时刻的行也不生效**（宁可不钉，不许永久钉住）。
func TestActive_TTL(t *testing.T) {
	now := time.Now()
	ok := Pin{ExpiresAt: now.Add(time.Minute).Format(time.RFC3339)}
	if !ok.Active(now) {
		t.Fatal("未到期的行该生效")
	}
	gone := Pin{ExpiresAt: now.Add(-time.Second).Format(time.RFC3339)}
	if gone.Active(now) {
		t.Fatal("已过期的行不该生效")
	}
	bad := Pin{ExpiresAt: "不是时刻"}
	if bad.Active(now) {
		t.Fatal("时刻读不出来的行**不该**生效（否则 = 永不过期 = 永久改变默认）")
	}
	if bad.Remaining(now) != 0 {
		t.Fatal("读不出时刻 ⇒ 剩余该是 0")
	}
}

// TestHostFor_ExpiredThenActiveRows —— 同一模型有两行（手改过件）时：**第一条生效的行**胜；
// 一行都不生效 ⇒ 不命中。写死的口径，不靠「谁后写谁赢」说话。
func TestHostFor_ExpiredThenActiveRows(t *testing.T) {
	now := time.Now()
	tbl := &Table{ID: TableID, Pins: []Pin{
		{Model: "m1", Machine: "旧的", ExpiresAt: now.Add(-time.Hour).Format(time.RFC3339)},
		{Model: "m1", Machine: "新的", ExpiresAt: now.Add(time.Hour).Format(time.RFC3339)},
	}}
	p, ok := tbl.HostFor("m1", now)
	if !ok || p.Machine != "新的" {
		t.Fatalf("该取第一条**生效**的行（新的），实得 %+v / %v", p, ok)
	}
	allExpired := &Table{ID: TableID, Pins: []Pin{
		{Model: "m1", Machine: "旧的", ExpiresAt: now.Add(-time.Hour).Format(time.RFC3339)},
	}}
	if _, ok := allExpired.HostFor("m1", now); ok {
		t.Fatal("全过期 ⇒ 不命中")
	}
}

// TestRemove_Prune_AndEmptyScope —— `Remove("")` = 撒全部；`Prune` 只清过期行并如实回吐。
func TestRemove_Prune_AndEmptyScope(t *testing.T) {
	now := time.Now()
	tbl := &Table{ID: TableID, Pins: []Pin{
		{Model: "m1", Machine: "x3", ExpiresAt: now.Add(time.Hour).Format(time.RFC3339)},
		{Model: "m2", Machine: "x3", ExpiresAt: now.Add(time.Hour).Format(time.RFC3339)},
		{Model: "m3", Machine: "x3", ExpiresAt: now.Add(-time.Minute).Format(time.RFC3339)},
	}}
	if got := tbl.Prune(now); len(got) != 1 || got[0].Model != "m3" {
		t.Fatalf("该只清掉过期的那一条（m3），实得 %+v", got)
	}
	if got := tbl.Remove("m1"); len(got) != 1 || got[0].Model != "m1" {
		t.Fatalf("该只撤掉 m1，实得 %+v", got)
	}
	if got := tbl.Remove(""); len(got) != 1 {
		t.Fatalf("`Remove(\"\")` 该撒全部（剩 m2 一条），实得 %+v", got)
	}
	if len(tbl.Pins) != 0 {
		t.Fatalf("撒完该是空表，实得 %+v", tbl.Pins)
	}
	if got := tbl.Remove("m1"); len(got) != 0 {
		t.Fatalf("幂等：没撒到东西该回空切片，实得 %+v", got)
	}
}

// TestLoad_NeverWrites —— **读者只读**：`Load`（含坏件那一档）跑前跑后，盘面 sha256 + mtime 逐字不变。
func TestLoad_NeverWrites(t *testing.T) {
	p := tmpTable(t)
	tbl := &Table{ID: TableID}
	tbl.Upsert(Pin{Model: "m1", Machine: "x3",
		ExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339), TTLSeconds: 3600})
	if err := tbl.Save(p); err != nil {
		t.Fatal(err)
	}
	st1, _ := os.Stat(p)
	for i := 0; i < 3; i++ {
		if _, err := Load(p); err != nil {
			t.Fatal(err)
		}
	}
	st2, _ := os.Stat(p)
	b1, _ := os.ReadFile(p)
	b2, _ := os.ReadFile(p)

	if string(b1) != string(b2) || !st1.ModTime().Equal(st2.ModTime()) || st1.Size() != st2.Size() {
		t.Fatalf("`Load` 动了盘面：%v/%d → %v/%d", st1.ModTime(), st1.Size(), st2.ModTime(), st2.Size())
	}
}

// TestPath_EnvOverride —— 落点解析：`ZERG_ROUTE_PINS` 最优先（测试 / 第二实例的隔离口）。
func TestPath_EnvOverride(t *testing.T) {
	want := filepath.Join(t.TempDir(), "自定义.json")
	t.Setenv(EnvPath, want)
	if got := Path(); got != want {
		t.Fatalf("环境变量该最优先：期望 %s，实得 %s", want, got)
	}
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	t.Setenv(EnvPath, "")
	if got := Path(); filepath.Base(got) != FileName {
		t.Fatalf("缺省该落在状态目录下、件名 %s，实得 %s", FileName, got)
	}
}
