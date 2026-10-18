package main

// zerg_egg_verify_bundle_test.go —— `GAP-20261015-16` 的成对判据：
// `cocoon verify` 的**复核束**必须覆盖「数据区首块 / 末块 / 索引体」。
//
// 四格（三负控 ＋ 一正控）：
//
//	① 正控：冷跑绿 ⇒ 二次跑命中缓存，且命中行**点名**三样读数（索引体 / 首块 / 末块）；
//	② 负控·首块：改数据区首块 1 字节 ＋ **复原 mtime**（size/inode 不变）⇒ 必红且点名「首块」；
//	③ 负控·末块：改数据区末块 1 字节 ＋ 复原 mtime ⇒ 必红且点名「末块」；
//	④ 负控·索引体：改尾段表 1 字节 ＋ 复原 mtime ⇒ 必红且点名「索引体」。
//
// ★ 复原 mtime 是每格的**前提**：不复原就退化成普通未命中（`st_mtime_ns 变`）⇒ 三条负控都不再能证明复核束有牙。
// ★ 夹具数据区必须 > 2 × `zegvBoundaryBlockBytes`（否则首块/末块重叠，末块负控会被首块那一条先吃下 ⇒ 分不出是哪一块）。

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// zegvTBundleFixture —— 数据区 3 MiB（6 × 512 KiB）的夹具：首块与末块**不重叠**，三块可各自独立取证。
func zegvTBundleFixture(t *testing.T, dir, name string) string {
	t.Helper()
	egg := zegvTFixture(t, dir, name, 6, 512)
	idx, err := zegParseIndex(egg)
	if err != nil {
		t.Fatalf("夹具索引解析失败：%v", err)
	}
	plan := zegvBlockPlanFromEntries(idx.Entries)
	if !plan.Has {
		t.Fatalf("夹具没有数据段")
	}
	if plan.HeadLen != int64(zegvBoundaryBlockBytes) || plan.TailLen != int64(zegvBoundaryBlockBytes) {
		t.Fatalf("夹具数据区不足两块：head=%d tail=%d（要各 %d）", plan.HeadLen, plan.TailLen, zegvBoundaryBlockBytes)
	}
	if plan.HeadOff == plan.TailOff {
		t.Fatalf("夹具首末块重叠：off=%d len=%d", plan.HeadOff, plan.HeadLen)
	}
	return egg
}

// zegvTBundleDataRange —— 数据区 `[start, end)`（只算 `size > 0` 的段）。
func zegvTBundleDataRange(idx *zegIndex) (int64, int64) {
	start, end := int64(-1), int64(-1)
	for _, e := range idx.Entries {
		if e.Size <= 0 {
			continue
		}
		if start < 0 || e.Off < start {
			start = e.Off
		}
		if end < 0 || e.Off+e.Size > end {
			end = e.Off + e.Size
		}
	}
	return start, end
}

// zegvTBundleTamperRun —— 冷跑（必绿、落条目）⇒ 改 1 字节 ⇒ 复原 mtime ⇒ 再跑。
// `pick` 拿盘上字节与索引，返回要翻转的**绝对偏移**。返回后一次的 rc/stdout/stderr。
func zegvTBundleTamperRun(t *testing.T, dir, name string, pick func(d []byte, idx *zegIndex) int64) (int, string, string) {
	t.Helper()
	egg := zegvTBundleFixture(t, dir, name)
	if rc, out, errb := zegvTRun(t, "verify", egg, "--workers", "2"); rc != exitOK {
		t.Fatalf("冷跑不绿：rc=%d out=%q err=%q", rc, out, errb)
	}
	fi, err := os.Stat(egg)
	if err != nil {
		t.Fatal(err)
	}
	d, err := os.ReadFile(egg)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := zegParseIndex(egg)
	if err != nil {
		t.Fatal(err)
	}
	off := pick(d, idx)
	if off < 0 || off >= int64(len(d)) {
		t.Fatalf("翻转偏移越界：%d（文件 %d 字节）", off, len(d))
	}
	d[off] ^= 0xff
	if err := os.WriteFile(egg, d, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(egg, fi.ModTime(), fi.ModTime()); err != nil {
		t.Fatal(err)
	}
	fi2, err := os.Stat(egg)
	if err != nil {
		t.Fatal(err)
	}
	if fi2.ModTime().UnixNano() != fi.ModTime().UnixNano() {
		t.Fatalf("mtime 未复原 ⇒ 本格退化成普通未命中（前提不成立）：%d ≠ %d", fi2.ModTime().UnixNano(), fi.ModTime().UnixNano())
	}
	if fi2.Size() != fi.Size() {
		t.Fatalf("st_size 变了 ⇒ 本格退化成普通未命中（前提不成立）：%d ≠ %d", fi2.Size(), fi.Size())
	}
	return zegvTRun(t, "verify", egg, "--workers", "2")
}

// ① 正控：命中行必须点名三样读数，且条目里首末块计划/摘要都是真的。
func TestZergEggVerifyBundlePositive(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(zegvCacheEnv, filepath.Join(dir, "cache"))
	egg := zegvTBundleFixture(t, dir, "bundle-pos")

	if rc, out, errb := zegvTRun(t, "verify", egg, "--workers", "2"); rc != exitOK {
		t.Fatalf("冷跑不绿：rc=%d out=%q err=%q", rc, out, errb)
	}
	rc, out, errb := zegvTRun(t, "verify", egg, "--workers", "2")
	if rc != exitOK {
		t.Fatalf("二次跑 rc=%d out=%q err=%q", rc, out, errb)
	}
	if !strings.Contains(out, zegvCacheMark) {
		t.Fatalf("二次跑未命中缓存：%q", out)
	}
	for _, key := range []string{"索引体=", "首块=", "末块="} {
		if !strings.Contains(out, key) {
			t.Fatalf("命中行没点名复核束读数 %q：%q", key, out)
		}
	}

	key, ok := zegvKeyFromBase(filepath.Base(egg))
	if !ok {
		t.Fatal("夹具名不是内容寻址形")
	}
	cdir, ok := zegvCacheDir()
	if !ok {
		t.Fatal("取不到缓存目录")
	}
	ent, why := zegvReadCache(zegvCachePath(cdir, key))
	if ent == nil {
		t.Fatalf("条目读不到：%s", why)
	}
	if ent.HeadLen != int64(zegvBoundaryBlockBytes) || ent.TailLen != int64(zegvBoundaryBlockBytes) {
		t.Fatalf("条目首末块长度不合：head=%d tail=%d", ent.HeadLen, ent.TailLen)
	}
	if len(ent.HeadSHA) != 64 || len(ent.TailSHA) != 64 || len(ent.IndexSHA) != 64 {
		t.Fatalf("条目摘要不是 64 位 hex：head=%q tail=%q index=%q", ent.HeadSHA, ent.TailSHA, ent.IndexSHA)
	}
	if ent.HeadSHA == ent.TailSHA {
		t.Fatalf("首末块摘要相同（夹具应让两块互不重叠）：%s", ent.HeadSHA)
	}
}

// ② 负控·首块：改数据区**首块** 1 字节（＋复原 mtime）⇒ 必红且点名「首块」。
func TestZergEggVerifyBundleHeadTamper(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(zegvCacheEnv, filepath.Join(dir, "cache"))
	rc, out, _ := zegvTBundleTamperRun(t, dir, "bundle-head", func(d []byte, idx *zegIndex) int64 {
		start, _ := zegvTBundleDataRange(idx)
		return start + 3
	})
	if rc == exitOK {
		t.Fatalf("首块被改（mtime 已复原）却判绿：%q", out)
	}
	if !strings.Contains(out, "首块") {
		t.Fatalf("红了但没点名「首块」：%q", out)
	}
	if strings.Contains(out, zegvCacheMark) {
		t.Fatalf("首块被改却仍命中缓存：%q", out)
	}
	t.Logf("红线：%s", strings.Join(strings.Split(strings.TrimSpace(out), "\n"), " | "))
}

// ③ 负控·末块：改数据区**末块** 1 字节（＋复原 mtime）⇒ 必红且点名「末块」。
func TestZergEggVerifyBundleTailTamper(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(zegvCacheEnv, filepath.Join(dir, "cache"))
	rc, out, _ := zegvTBundleTamperRun(t, dir, "bundle-tail", func(d []byte, idx *zegIndex) int64 {
		_, end := zegvTBundleDataRange(idx)
		return end - 1
	})
	if rc == exitOK {
		t.Fatalf("末块被改（mtime 已复原）却判绿：%q", out)
	}
	if !strings.Contains(out, "末块") {
		t.Fatalf("红了但没点名「末块」：%q", out)
	}
	if strings.Contains(out, zegvCacheMark) {
		t.Fatalf("末块被改却仍命中缓存：%q", out)
	}
	t.Logf("红线：%s", strings.Join(strings.Split(strings.TrimSpace(out), "\n"), " | "))
}

// ④ 负控·索引体：改**尾段表** 1 字节（＋复原 mtime）⇒ 必红且点名「索引体」。
func TestZergEggVerifyBundleIndexTamper(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(zegvCacheEnv, filepath.Join(dir, "cache"))
	rc, out, _ := zegvTBundleTamperRun(t, dir, "bundle-index", func(d []byte, idx *zegIndex) int64 {
		// v2 = 尾段表；v1 = 前缀后紧跟的变长索引体（`zegPack` 产的就是 v1）⇒ 两种布局各自定点取索引体。
		if idx.TableOff > 0 && idx.TableLen > 0 {
			return idx.TableOff + 8
		}
		n := int64(binary.LittleEndian.Uint64(d[16:24]))
		if n <= 8 || int64(zegHeaderPrefix)+8 >= int64(len(d)) {
			t.Fatalf("夹具索引体长度不合理：%d（文件 %d 字节）", n, len(d))
		}
		return int64(zegHeaderPrefix) + 8
	})
	if rc == exitOK {
		t.Fatalf("索引体被改（mtime 已复原）却判绿：%q", out)
	}
	if !strings.Contains(out, "索引体") {
		t.Fatalf("红了但没点名「索引体」：%q", out)
	}
	if strings.Contains(out, zegvCacheMark) {
		t.Fatalf("索引体被改却仍命中缓存：%q", out)
	}
	t.Logf("红线：%s", strings.Join(strings.Split(strings.TrimSpace(out), "\n"), " | "))
}
